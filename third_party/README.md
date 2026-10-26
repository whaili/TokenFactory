# 开源推理引擎源码

通过 `git subtree --squash` 引入官方仓库源码，供阅读、调研和实验使用。
导入日期：2026-10-26；以下版本为导入时 `main` 分支的快照。

| 项目 | 本地目录 | 官方仓库 | 上游提交 | 许可证 |
|---|---|---|---|---|
| vLLM | [vllm](vllm/) | [vllm-project/vllm](https://github.com/vllm-project/vllm) | `0f6b26191064384aa35060c87f4ed570c7875e66` | Apache-2.0 |
| SGLang | [sglang](sglang/) | [sgl-project/sglang](https://github.com/sgl-project/sglang) | `66cac6b79ccffa13c108297fad8a43ddc49d06f8` | Apache-2.0 |
| DeepInfer | [DeepInfer](DeepInfer/) | [deepseek-ai/DeepInfer](https://github.com/deepseek-ai/DeepInfer) | `553741557a57df44a46fb3ab72b6aafeaf945337` | Apache-2.0 |

DeepInfer 是截至导入日 DeepSeek 最新开源的推理框架，官方仓库创建于
2026-10-16。当前开源版本提供离线推理，支持 DeepSeek-V4.1-Flash，
运行要求与功能边界见其 [README](DeepInfer/README.md)。

## 同步上游

在本仓库根目录、工作区干净时执行：

```bash
git subtree pull --prefix=third_party/vllm https://github.com/vllm-project/vllm.git main --squash
git subtree pull --prefix=third_party/sglang https://github.com/sgl-project/sglang.git main --squash
git subtree pull --prefix=third_party/DeepInfer https://github.com/deepseek-ai/DeepInfer.git main --squash
```

同步后更新上表中的提交和日期。精确上游版本也记录在 subtree 提交的
`git-subtree-split` 字段中。

## 使用与许可证

普通 `git clone` 即可取得这三个项目的源码，无需初始化 submodule。
本次仅引入源码；构建依赖、模型权重和运行环境请按照各项目文档准备。
上游自身的子模块及外部依赖仍需按各自说明获取。

第三方源码保留各自许可证及版权声明，不适用本仓库原创内容的 MIT 许可证：

- [vLLM LICENSE](vllm/LICENSE)
- [SGLang LICENSE](sglang/LICENSE)
- [DeepInfer LICENSE](DeepInfer/LICENSE)
