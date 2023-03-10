/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cmd

//import logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
//
//var log = logf.Log.WithName("cmd")

// 命令初始化
func CmdInit() *baseCommand {
	cli := NewCli()
	baseCmd := &baseCommand{
		command: cli.rootCmd,
	}
	// 增加Version子命令
	// add version command
	baseCmd.AddCommand(&VersionCommand{})
	// 增加prepare子命令
	// add prepare command
	prepareCommand := &PrepareCommand{}
	baseCmd.AddCommand(prepareCommand)
	// 给prepare子命令再增加子命令
	prepareCommand.AddCommand(&PrepareJvmCommand{})
	prepareCommand.AddCommand(&PrepareCPlusCommand{})

	// 增加 revoke 子命令
	// add revoke command
	baseCmd.AddCommand(&RevokeCommand{})

	// 增加 create 子命令
	// add create command
	createCommand := &CreateCommand{}
	baseCmd.AddCommand(createCommand)

	// 增加 destroy 子命令
	// add destroy command
	destroyCommand := &DestroyCommand{}
	baseCmd.AddCommand(destroyCommand)

	// 增加 status 子命令
	// add status command
	baseCmd.AddCommand(&StatusCommand{})

	// add query command
	queryCommand := &QueryCommand{}
	baseCmd.AddCommand(queryCommand)
	queryCommand.AddCommand(&QueryDiskCommand{})
	queryCommand.AddCommand(&QueryNetworkCommand{})
	queryCommand.AddCommand(&QueryJvmCommand{})
	queryCommand.AddCommand(&QueryK8sCommand{})

	// add server command
	serverCommand := &ServerCommand{}
	baseCmd.AddCommand(serverCommand)
	serverCommand.AddCommand(&StartServerCommand{})
	serverCommand.AddCommand(&StopServerCommand{})
	serverCommand.AddCommand(&StatusServerCommand{})

	// add check command
	checkCommand := &CheckCommand{}
	baseCmd.AddCommand(checkCommand)
	checkCommand.AddCommand(&CheckJavaCommand{})
	checkCommand.AddCommand(&CheckOsCommand{})

	return baseCmd
}
