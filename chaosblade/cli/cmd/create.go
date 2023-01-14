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

/*
官方create的文档：https://chaosblade-io.gitbook.io/chaosblade-help-zh-cn/blade-create
*/

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chaosblade-io/chaosblade-spec-go/log"
	"github.com/shirou/gopsutil/process"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/spf13/pflag"

	"github.com/chaosblade-io/chaosblade/data"

	"github.com/chaosblade-io/chaosblade-spec-go/channel"
	"github.com/chaosblade-io/chaosblade-spec-go/spec"
	"github.com/chaosblade-io/chaosblade-spec-go/util"
	"github.com/spf13/cobra"
)

// CreateCommand for create experiment
type CreateCommand struct {
	baseCommand //即*cobra.Command
	*baseExpCommandService
	async bool // Whether to create asynchronously, default is false
	// Actively report the create result.
	// The installation result report is triggered only when the async value is true and the value is not empty.
	endpoint string
	nohup    bool //used to internal async create, no need to config
}

const UidFlag = "uid"
const AsyncFlag = "async"
const EndpointFlag = "endpoint"
const NohupFlag = "nohup"

var uid string

//子命令create初始化
func (cc *CreateCommand) Init() {
	cc.command = &cobra.Command{
		Use:     "create",
		Short:   "Create a chaos engineering experiment",
		Long:    "Create a chaos engineering experiment",
		Aliases: []string{"c"},   //通过blade c也能创建实验
		Example: createExample(), //使用范例，会展示在帮助文档的Examples栏里
	}
	flags := cc.command.PersistentFlags()
	//每个实验对应一个 uid，后续的查询、销毁实验都要用到此 uid
	flags.StringVar(&uid, UidFlag, "", "Set Uid for the experiment, adapt to docker and cri")
	flags.BoolVarP(&cc.async, AsyncFlag, "a", false, "whether to create asynchronously, default is false")
	//todo：[同时给出--async，此参数才会有效???实际测试也会发送请求]
	//配置callback的endpoint，如--endpoint=http://127.0.0.1。创建等动作会给此endpoint发送POST回调
	flags.StringVarP(&cc.endpoint, EndpointFlag, "e", "", "the create result reporting address. It takes effect only when the async value is true and the value is not empty")
	flags.BoolVarP(&cc.nohup, NohupFlag, "n", false, "used to internal async create, no need to config")

	cc.baseExpCommandService = newBaseExpCommandService(cc)
}

// 将规范的命令注册到cobra.Command中，并将Flag的值保存到commandFlags[flagName]中
func (cc *CreateCommand) bindFlagsFunction() func(commandFlags map[string]func() string, cmd *cobra.Command, specFlags []spec.ExpFlagSpec) {
	return func(commandFlags map[string]func() string, cmd *cobra.Command, specFlags []spec.ExpFlagSpec) {
		// set action flags
		for _, flag := range specFlags {
			flagName := flag.FlagName() //得到参数名称
			flagDesc := flag.FlagDesc() //得到参数描述
			//检查强制Flag参数：如果这个参数是强制的，但又没有设置，则提示用户并退出。
			if flag.FlagRequired() {
				flagDesc = fmt.Sprintf("%s (required)", flagDesc)
				cmd.MarkPersistentFlagRequired(flagName)
			}
			//将Bool和String的Flag参数都转化成String保存在commandFlags[flagName]中
			if flag.FlagNoArgs() {
				var key bool
				cmd.PersistentFlags().BoolVar(&key, flagName, false, flagDesc)
				commandFlags[flagName] = func() string {
					return strconv.FormatBool(key) //将bool转化成string返回
				}
			} else {
				var key string
				cmd.PersistentFlags().StringVar(&key, flagName, flag.FlagDefault(), flagDesc)
				commandFlags[flagName] = func() string {
					return key
				}
			}
		}
	}
}

func (cc *CreateCommand) actionRunEFunc(target, scope string, actionCommand *actionCommand, actionCommandSpec spec.ExpActionCommandSpec) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		expModel := createExpModel(target, scope, actionCommandSpec.Name(), cmd)
		expModel.ActionProcessHang = actionCommandSpec.ProcessHang()
		// check timeout flag
		tt := expModel.ActionFlags["timeout"]
		if tt != "" {
			//errNumber checks whether timout flag is parsable as Number
			if _, errNumber := strconv.ParseUint(tt, 10, 64); errNumber != nil {
				//err checks whether timout flag is parsable as Time
				if _, err := time.ParseDuration(tt); err != nil {
					return err
				}
			}
		}
		nohup := expModel.ActionFlags[NohupFlag] == "true"
		var model *data.ExperimentModel
		var resp *spec.Response
		var err error
		ctx := context.Background()

		if nohup {
			uid := expModel.ActionFlags[UidFlag]
			if uid == "" {
				ctx := context.Background()
				log.Infof(ctx, "can not execute nohup, uid is null")
				return spec.ResponseFailWithFlags(spec.ParameterLess, UidFlag)
			} else {
				ctx = context.WithValue(context.Background(), spec.Uid, uid)
				model, err = GetDS().QueryExperimentModelByUid(uid)
				if err == nil {
					delete(expModel.ActionFlags, NohupFlag)
				}
			}
		} else {
			// update status
			model, resp = actionCommand.recordExpModel(cmd.CommandPath(), expModel)
		}
		if resp != nil && !resp.Success {
			return resp
		}
		// is async ?
		async := expModel.ActionFlags[AsyncFlag] == "true"
		endpoint := expModel.ActionFlags[EndpointFlag]

		if async {
			var args string
			if scope == "host" {
				args = fmt.Sprintf("create %s %s --uid %s --nohup=true", target, actionCommand.Name(), model.Uid)
			} else if scope == "docker" || scope == "cri" {
				args = fmt.Sprintf("create %s %s %s --uid %s --nohup=true", scope, target, actionCommand.Name(), model.Uid)
			} else {
				args = fmt.Sprintf("create k8s %s-%s %s --uid %s --nohup=true", scope, target, actionCommand.Name(), model.Uid)
			}
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				if flag.Value.String() == "false" {
					return
				}
				if flag.Name == AsyncFlag || flag.Name == UidFlag {
					return
				}
				args = fmt.Sprintf("%s --%s=%s ", args, flag.Name, flag.Value)
			})
			args = fmt.Sprintf("%s %s %s", path.Join(util.GetProgramPath(), "blade"), args, "> /dev/null 2>&1 &")
			response := channel.NewLocalChannel().Run(context.Background(), "nohup", args)
			if response.Success {
				log.Infof(ctx, "async create success, uid: %s", model.Uid)
				cmd.Println(spec.ReturnSuccess(model.Uid).Print())
			} else {
				log.Warnf(ctx, "async create fail, err: %s, uid: %s", response.Err, model.Uid)
				cmd.Println(spec.ResponseFailWithFlags(spec.OsCmdExecFailed, "nohup", response.Err).Print())
			}
			return nil
		} else {
			// execute experiment
			executor := actionCommandSpec.Executor()
			executor.SetChannel(channel.NewLocalChannel())
			ctx := context.WithValue(context.Background(), spec.Uid, model.Uid)
			response := executor.Exec(model.Uid, ctx, expModel)
			if response.Code == spec.ReturnOKDirectly.Code {
				// return directly
				response.Code = spec.OK.Code
				cmd.Println(response.Print())
				endpointCallBack(ctx, endpoint, model.Uid, response)
			}
			// pass the uid, expModel to actionCommand
			actionCommand.expModel = expModel
			actionCommand.uid = model.Uid

			if !response.Success {
				// update status
				checkError(GetDS().UpdateExperimentModelByUid(model.Uid, Error, response.Err))
				endpointCallBack(ctx, endpoint, model.Uid, response)
				return response
			}

			if expModel.ActionProcessHang && scope != "pod" && scope != "container" && scope != "node" && expModel.ActionFlags["channel"] != "ssh" {
				// todo -> need to find a better way to query the status
				time.Sleep(time.Millisecond * 100)
				log.Debugf(ctx, "result: %v", response.Result)
				if response.Result == nil {
					errMsg := fmt.Sprintf("chaos_os process not found, please check chaosblade log")
					checkError(GetDS().UpdateExperimentModelByUid(model.Uid, Error, errMsg))
					response.Err = errMsg
				} else {
					_, err := process.NewProcess(int32(response.Result.(int)))
					if err != nil {
						errMsg := fmt.Sprintf("chaos_os process not found, please check chaosblade log, err: %s", err.Error())
						checkError(GetDS().UpdateExperimentModelByUid(model.Uid, Error, errMsg))
						response.Err = errMsg
					} else {
						// update status
						checkError(GetDS().UpdateExperimentModelByUid(model.Uid, Success, response.Err))
					}
				}
			} else {
				// update status
				checkError(GetDS().UpdateExperimentModelByUid(model.Uid, Success, response.Err))
			}
			response.Result = model.Uid
			cmd.Println(response.Print())
			endpointCallBack(ctx, endpoint, model.Uid, response)
			return nil
		}
	}
}

//给endpoint发送回调POST请求
func endpointCallBack(ctx context.Context, endpoint, uid string, response *spec.Response) {
	if endpoint != "" {
		log.Infof(ctx, "report response: %s to endpoint: %s", response.Print(), endpoint)
		experimentModel, _ := GetDS().QueryExperimentModelByUid(uid)
		body, err := json.Marshal(experimentModel)
		if err != nil {
			log.Warnf(ctx, "create post body %s failed, %v", response.Print(), err)
		} else {
			result, err, code := util.PostCurl(endpoint, body, "application/json")
			if err != nil {
				log.Warnf(ctx, "report result %s failed, %v", response.Print(), err)
			} else if code != 200 {
				log.Warnf(ctx, "response code is %d, result %s", code, result)
			} else {
				log.Infof(ctx, "report result success, result %s", result)
			}
		}
	}
}

func (cc *CreateCommand) actionPostRunEFunc(actionCommand *actionCommand) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		const bladeBin = "blade"
		if actionCommand.expModel != nil {
			tt := actionCommand.expModel.ActionFlags["timeout"]
			async := actionCommand.expModel.ActionFlags[AsyncFlag] == "true"
			if tt == "" || async {
				return nil
			}
			//err possible if timeout used as timeDuration.
			timeout, err := strconv.ParseUint(tt, 10, 64)

			if err != nil {
				// the err checked in RunE function
				timeDuartion, _ := time.ParseDuration(tt)
				timeout = uint64(timeDuartion.Seconds())
			}

			if timeout > 0 && actionCommand.uid != "" {
				// fix https://github.com/chaosblade-io/chaosblade-operator/issues/34
				if actionCommand.expModel.Scope == "container" || actionCommand.expModel.Scope == "pod" {
					timeout = timeout + 60
				}
				script := path.Join(util.GetProgramPath(), bladeBin)
				args := fmt.Sprintf("nohup /bin/sh -c 'sleep %d; %s destroy %s' > /dev/null 2>&1 &",
					timeout, script, actionCommand.uid)
				cmd := exec.CommandContext(context.TODO(), "/bin/sh", "-c", args)
				return cmd.Run()
			}
		}
		return nil
	}
}

//create命令的使用范例，会展示在帮助文档的Examples栏里
func createExample() string {
	return `blade create cpu load --cpu-percent 60`
}
