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

import (
	"github.com/spf13/cobra"

	"github.com/chaosblade-io/chaosblade/version"
)

type VersionCommand struct {
	baseCommand
}

func (vc *VersionCommand) Init() {
	vc.command = &cobra.Command{
		Use:     "version", // 输入balde version会执行此命令
		Short:   "Print version info",
		Long:    "Print version info",
		Aliases: []string{"v"}, //通过输入balde v也能执行此命令
		Run: func(cmd *cobra.Command, args []string) { // 输入balde version会执行此函数
			cmd.Printf("version: %s\n", version.Ver)
			cmd.Printf("env: %s\n", version.Env)
			cmd.Printf("build-time: %s\n", version.BuildTime)
			return
		},
	}
}
