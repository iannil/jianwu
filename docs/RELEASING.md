# 本地发布构建

需要 Go 1.25+、Git 和 `shasum`。开发构建显示 `internal/cli/version.go` 中管理的版本（当前 `0.3.7`），commit 和构建时间显示 `unknown`。`jianwu --version`、`jianwu -v` 和 `jianwu version` 输出相同信息。

在仓库根目录验证当前源码：

```sh
scripts/release_test.sh
scripts/release.sh --dry-run
```

VERSION 可省略，默认取 `internal/cli/version.go` 的 `var Version`（单一版本来源）；也可显式传入覆盖。格式为 `MAJOR.MINOR.PATCH`，不带 `v` 前缀，不接受 `-dev` 等预发布后缀。dry-run 允许未提交的改动，commit 会带 `-dirty`；它仍执行完整 `go test -race ./...`、`go vet ./...` 和真实构建，并逐一校验三个版本入口的输出。

准备正式本地产物时：

```sh
scripts/release.sh
```

此模式要求工作区干净（包括未跟踪文件），并拒绝已存在的同名本地 `v<VERSION>` 标签。两个模式都只构建当前操作系统和架构的可执行文件，均不会创建标签、提交、推送或发布。脚本对 HEAD、Git 索引、已跟踪文件和未被忽略的未跟踪文件计算内容指纹，在测试检查后及构建后分别比较；内容变化会终止发布。文件名中的空格和换行、符号链接目标及可执行标记均纳入检查。暂不支持含 Git 子模块的发布检出。运行期间请勿编辑源码或切换提交。

脚本打印临时目录中的 `jianwu` 绝对路径。目录同时保留 `SHA256SUMS` 和 `BUILD_INFO`；验证失败时清理未完成产物。使用完毕后可自行删除此目录。构建元数据由版本（显式参数或从 version.go 派生）、Git HEAD 和 HEAD 的提交时间组成；`built` 使用提交时间而非墙上时钟，以便相同源码构建获得稳定的元数据。dry-run 产物包含未提交改动，仅供本地验证，不能作为可追溯的正式发布。

如需指定可写的 Go 缓存，可在命令前设置 `GOCACHE=/absolute/writable/path`。分发、签名、跨平台打包和远端发布需要另行执行。
