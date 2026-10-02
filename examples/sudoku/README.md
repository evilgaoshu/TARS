# 数独示例 · `examples/sudoku`

EVI-142 交付的可玩网页数独：9×9 棋盘、固定唯一解题面、输入/清除、行/列/宫重复提示、
完成判定、重置、新游戏。规格见 [`SPEC.md`](./SPEC.md)。

本目录是**独立示例**：不依赖 TARS 既有业务代码，也没有 `npm install` 步骤。

## 环境要求

| 工具 | 版本（实测） | 用途 |
|---|---|---|
| Node.js | v26.10.0（建议 ≥ 20） | 测试、题面生成、场景脚本 |
| Python 3 | 3.13.12（建议 ≥ 3.8） | 独立复核唯一解 |
| 静态服务器 | `python3 -m http.server` | 启动页面（任何静态服务器均可） |

## 安装

```bash
# 无需安装依赖：纯 Node 内置模块 + Python 标准库 + 原生 HTML/CSS/JS
git clone https://github.com/evilgaoshu/TARS
cd TARS
```

## 启动

```bash
# 方式一：仓库根目录
python3 -m http.server 8142 --directory examples/sudoku

# 方式二：进入目录
cd examples/sudoku && python3 -m http.server 8142
```

浏览器打开 <http://127.0.0.1:8142/> 即可游玩。

## 测试

所有命令都在**仓库根目录**执行。

### 1. 题面生成（确定性，可重复执行）

```bash
node examples/sudoku/generate-fixtures.mjs
```

```
easy-seed-1421  clues=40  empties=41  unique=true  seed=1421
medium-seed-7331  clues=34  empties=47  unique=true  seed=7331
hard-seed-9091  clues=28  empties=53  unique=true  seed=9091

wrote 3 fixtures -> examples/sudoku/fixtures.json
```

CI 另外执行 `git diff --exit-code -- examples/sudoku/fixtures.json` 保证字节级可复现。

### 2. 行为契约测试（43 例）

```bash
node --test examples/sudoku/
```

```
ℹ tests 43
ℹ suites 0
ℹ pass 43
ℹ fail 0
ℹ cancelled 0
ℹ skipped 0
ℹ todo 0
ℹ duration_ms 152.051416
```

覆盖：行/列/宫冲突（含同格同时触发行+列的去重、空格不参与）、正确完成、未填满、
填满但有冲突不算完成、重置恢复题面、题目格 input/clear 抛错且不改棋盘、
空格输入 1–9 与非法值被拒、新游戏切换、题目合法且唯一解。

### 3. 独立复核（Python，不同实现路径）

```bash
python3 examples/sudoku/verify_fixtures.py
```

```
  [PASS] easy-seed-1421: puzzle legal (row/col/box)
  [PASS] easy-seed-1421: unique solution (found 1)
  [PASS] medium-seed-7331: puzzle legal (row/col/box)
  [PASS] medium-seed-7331: unique solution (found 1)
  [PASS] hard-seed-9091: puzzle legal (row/col/box)
  [PASS] hard-seed-9091: unique solution (found 1)
  ...（每题 8 项，共 24 项全 PASS）

INDEPENDENT VERIFY OK: 3/3 fixtures re-confirmed by Python solver (legal + exactly one solution)
```

### 4. 验收场景脚本（15 项）

```bash
node examples/sudoku/e2e-scenario.mjs
```

```
[PASS] 9x9 棋盘共 81 格
[PASS] 题目格数量与 fixture 一致        observed: 40 givens
[PASS] 所有题目格都拒绝修改            observed: 40/40 rejected
[PASS] 空格 (0,1) 可输入 1-9           observed: value=7, conflicts=[]
[PASS] 空格可清除回空                  observed: value=0
[PASS] 行内重复产生冲突提示            observed: conflicts=[0]
[PASS] 列内重复产生冲突提示            observed: conflicts=[0]
[PASS] 宫内重复产生冲突提示            observed: conflicts=[0]
[PASS] 正确填满显示完成                observed: empties=0, complete=true
[PASS] 未填满不显示完成                observed: empties=40
[PASS] 重置恢复题面                    observed: clues=40, complete=false
[PASS] 新游戏切换到新题面              observed: fixture=medium-seed-7331, clues=34
[PASS] 题目 easy-seed-1421 合法且唯一解  observed: solutions=1, clues=40
[PASS] 题目 medium-seed-7331 合法且唯一解 observed: solutions=1, clues=34
[PASS] 题目 hard-seed-9091 合法且唯一解  observed: solutions=1, clues=28

--- 固定题面（随后逐题打印 puzzle / answer，见 fixtures.json） ---

=== 结果: 15/15 PASS, 0 FAIL ===
```

### 5. 仓库整体检查（PR 的 CI 同款）

```bash
make pre-check        # Go 编译 + OpenAPI 校验
make check-mvp        # go test/build + OpenAPI + web lint/test/build
make secret-scan      # 发布树密钥扫描
make security-regression
```

### 6. CI

PR 触发既有 `mvp-checks.yml`（5 个 job）与 `sudoku-example.yml`
（题面幂等 + 43 例行为测试 + Python 复核 + 15 项场景脚本）。

## 浏览器手工验收步骤

1. 启动本地服务后打开 <http://127.0.0.1:8142/>。
2. **9×9 棋盘**：81 格，题面 40 个题目格（浅色加粗）。
3. **题目格不可改**：点击第 1 行第 1 列，状态条提示「题目格，不可修改」，值仍为 6。
4. **空格输入/清除**：点一个空格，键盘按 7 → 填入；Backspace 或「清除」→ 回到空。
5. **行/列/宫提示**：填入与同行、同列、同宫重复的数字，对应冲突格变红，状态条提示重复。
6. **完成**：填满全部空格且无冲突 → 状态「已完成」并出现完成横幅。
7. **重置**：点「重置」→ 恢复题面，用户格清空，计时归零。
8. **新游戏**：点「新游戏」或用「切换题面」下拉 → 切换到另一题面（中等/偏难）。
9. **键盘与手机**：方向键移动、1–9 填写、Backspace 删除；窄屏下棋盘与数字键盘纵向排列。

### 自动化钩子（取证 / 复测用）

页面暴露 `window.__sudoku`：`state`（只读状态快照）、`select(r,c)`、`input(r,c,v)`、
`clear(r,c)`、`conflictsAt(r,c)`、`reset()`、`newGame()`、`loadFixture(id)`、`solveToEnd()`。
浏览器自动化可直接读取 `state.complete` / `state.conflicts` 断言结果，无需解析 DOM 文本。

## 目录结构

```
examples/sudoku/
├── SPEC.md                       # 规格（本 issue 的验收映射）
├── README.md                     # 本文件：安装/启动/测试
├── index.html                    # 页面骨架
├── app.js                        # 交互层（渲染 + 输入路由 + 自动化钩子）
├── style.css                     # 样式（含手机断点）
├── rules.mjs                     # 规则判定：单元冲突 / 唯一解计数 / 确定性生成
├── game.mjs                      # 状态机：只读题目格 / 输入 / 清除 / 重置 / 新游戏 / 完成
├── fixtures.json                 # 3 个固定题面 + 答案（字节级可复现）
├── generate-fixtures.mjs         # 题面生成器（固定 seed）
├── verify_fixtures.py            # 独立 Python 复核（不同代码路径）
├── sudoku.conformance.test.mjs   # 43 条行为契约用例
└── e2e-scenario.mjs              # 15 项验收场景脚本
```

## 来源与贡献

- `rules.mjs`、`game.mjs`、`fixtures.json`、`generate-fixtures.mjs`、`verify_fixtures.py`、
  `sudoku.conformance.test.mjs`、`e2e-scenario.mjs` 来自协作 DEV（DEV-PI）的
  `sudoku-verification` 交付（issue 评论 `01a0faf1-e0f4-77c0-bbef-d99f58d7324f` 附件），
  按其整合建议原样接入（仅把注释里的目录路径改为 `examples/sudoku/`）。
- 前端（`index.html` / `app.js` / `style.css`）、`SPEC.md`、`README.md`、
  `.github/workflows/sudoku-example.yml` 由主 DEV（DEV-mac-opencode）产出。
