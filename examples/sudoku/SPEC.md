# SPEC · 简单数独（EVI-142）

- 仓库：`evilgaoshu/TARS`（test 项目）
- 位置：`examples/sudoku/`（独立示例目录，不改动 TARS 既有业务代码）
- 验收 issue：EVI-142 — 6 agent 配置更新后的全流程验证

## 1. 目标

交付一个**可玩的网页数独**：9×9 棋盘、固定唯一解题面、输入/清除/冲突提示/完成判定/重置/新游戏，
并以一次真实 PR 完成开发 → QA → 合并 → 复盘的全流程验证。

## 2. 范围

### 2.1 必须（hard requirements）

| # | 需求 | 实现位置 |
|---|---|---|
| R1 | README 给出安装、启动、测试命令，逐条执行成功并记录输出/URL | `README.md` |
| R2 | 浏览器显示 9×9 棋盘 | `index.html` + `app.js` `buildBoard()` |
| R3 | 题目格（given）不可修改 | `game.mjs` `input/clear` 抛错 + `<input readonly>` |
| R4 | 空格可输入、可清除 1–9 | `game.mjs` `input/clear`、`app.js` `applyDigit/applyClear` |
| R5 | 行、列、宫重复有提示 | `rules.mjs` `conflictCells/conflictsAt` + `.conflict` 高亮 |
| R6 | 正确填满显示完成 | `game.mjs` `isComplete`（= 满 + 无冲突）+ 完成横幅 |
| R7 | 重置恢复题面 | `game.mjs` `reset` |
| R8 | 新游戏可开始 | `game.mjs` `newGame` + 「新游戏」按钮/题面下拉 |
| R9 | 自动测试覆盖行/列/宫冲突、正确完成、未填满、重置、题目合法且唯一解 | `sudoku.conformance.test.mjs`（43 例） |
| R10 | CI 全部 SUCCESS | `.github/workflows/sudoku-example.yml` + 既有 `mvp-checks.yml` |
| R11 | SPEC 进入版本控制；PR 标题含 `EVI-142` | 本文件 |
| R12 | DEV 与 QA 各自给出命令输出与浏览器截图，注明同一 head | PR / issue 评论 |

### 2.2 best-effort

手机可玩（响应式 + 数字键盘）、键盘操作（方向键 / 数字 / Backspace）、计时显示。

### 2.3 不做

账号、联网对战、排行榜；不修改 TARS 既有业务；仅一次流程测试，不自动派生下一开发任务。

## 3. 规则与状态语义

网格为长度 81 的数组，`i = r*9+c`，`0` 表示空格。

- **单元**：9 行 + 9 列 + 9 宫，共 27 个；任一单元内数字不得重复。
- **冲突提示**：`conflictsAt(grid, r, c)` 返回与该格同单元且同数字的**其它格**索引；
  全盘冲突格 = `conflictCells(grid)`。UI 高亮这些格即满足 R5。
- **完成判定**：`isValidFull(board)` = 每格 1–9 且 27 个单元均无重复（填满 + 无冲突）。
  对唯一解题面，与「逐格等于答案」等价。
- **题目格只读**：题面里非 0 的格子；`input/clear` 对其抛错且不改变棋盘，UI 侧同时 `readonly`。
- **唯一解前置校验**：题面进入 UI 前必须通过 `assertPlayableFixture`（合法 + 与答案一致 + `countSolutions(puzzle, 2) === 1`），
  否则「正确填满」存在歧义。
- **重置 / 新游戏**：`reset` 恢复当前题面（题目格不变、用户格清空）；`newGame` 切换题面与答案。

## 4. 固定题面

`fixtures.json` 由 `generate-fixtures.mjs` 以固定 seed 确定性生成，字节级可复现：

| id | 提示数 | 空格 | seed |
|---|---|---|---|
| easy-seed-1421 | 40 | 41 | 1421 |
| medium-seed-7331 | 34 | 47 | 7331 |
| hard-seed-9091 | 28 | 53 | 9091 |

三题均 `uniqueSolution: true`，由 JS（`countSolutions`）与 Python（`verify_fixtures.py`，独立实现）双重复核。

## 5. 测试策略

| 层 | 命令 | 覆盖 |
|---|---|---|
| 行为契约 | `node --test examples/sudoku/` | 行/列/宫冲突、正确完成、未填满、填满但有冲突、重置、题目格只读、输入/清除、新游戏、题目合法且唯一解（43 例） |
| 独立复核 | `python3 examples/sudoku/verify_fixtures.py` | 与 JS 不同代码路径复核唯一解与合法性 |
| 场景脚本 | `node examples/sudoku/e2e-scenario.mjs` | R2–R8 验收项逐条 PASS/FAIL + 观察值 |
| 题面幂等 | `node examples/sudoku/generate-fixtures.mjs` 后 `git diff --exit-code` | 题面字节级可复现 |
| 浏览器实测 | 起本地服务 + 浏览器操作/截图 | 真实渲染与交互，DEV 与 QA 各自取证 |
| CI | push/PR 触发 | 既有 5 个 check + `sudoku-example` workflow |

## 6. 交付与流转

1. DEV：实现 + 本地验证 + 浏览器截图 → PR（标题含 `EVI-142`）→ 取得 CI 结果 → 评论转交 QA。
2. QA：在**同一 head** 独立复跑命令与浏览器探索性测试，给出截图与结论。
3. PM：核对同一 head 与 checks 全 SUCCESS 后仅 merge 一次。
4. RETRO：单次复盘，逐 agent 列 run ID 与职责产出。

## 7. 参考实现来源

`rules.mjs`、`game.mjs`、`fixtures.json`、`generate-fixtures.mjs`、`verify_fixtures.py`、
`sudoku.conformance.test.mjs`、`e2e-scenario.mjs` 来自协作 DEV（DEV-PI）的
`sudoku-verification` 交付（issue 附件 `sudoku-verification-devpi.tar.gz`，评论 `01a0faf1-e0f4-77c0-bbef-d99f58d7324f`），
按其整合建议接入；前端、SPEC、README、CI workflow 由主 DEV（DEV-mac-opencode）产出。
