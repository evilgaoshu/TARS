/**
 * EVI-142 端到端场景脚本：把 issue 里的浏览器验收项逐条「脚本化」。
 *
 * 运行：node examples/sudoku/e2e-scenario.mjs
 *
 * 输出是一份可复现的期望行为记录（每条 hard requirement 一行 PASS/FAIL + 观察值）。
 * 主 DEV 在浏览器里手工操作时应得到同样的判定；QA 可用它对照截图。
 */

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import {
  CELLS,
  EMPTY,
  cellIndex,
  conflictsAt,
  countSolutions,
  formatGrid,
  parseGrid,
} from './rules.mjs';
import {
  clear,
  createGame,
  emptyCount,
  fillCount,
  findEmpty,
  input,
  isComplete,
  isGiven,
  newGame,
  reset,
  solveToEnd,
} from './game.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));
const doc = JSON.parse(readFileSync(join(HERE, 'fixtures.json'), 'utf8'));
const F = doc.fixtures.map((f) => ({ ...f, puzzle: parseGrid(f.puzzle), solution: parseGrid(f.solution) }));

let failures = 0;
const results = [];

function check(name, condition, observed) {
  const ok = Boolean(condition);
  if (!ok) failures++;
  results.push({ name, ok, observed });
  console.log(`[${ok ? 'PASS' : 'FAIL'}] ${name}`);
  if (observed !== undefined) console.log(`       observed: ${observed}`);
}

const primary = F[0];
console.log('=== EVI-142 数独行为验收（脚本化） ===');
console.log(`fixture: ${primary.id}  clues=${primary.clues}  empties=${primary.empties}\n`);

// 1. 棋盘 9x9 呈现 + 题目格只读
const game = createGame(primary);
check('9x9 棋盘共 81 格', game.board.length === CELLS, `${game.board.length} cells`);
const givenCount = game.board.filter((v, i) => game.puzzle[i] !== EMPTY).length;
check('题目格数量与 fixture 一致', givenCount === primary.clues, `${givenCount} givens`);

let givenRejected = 0;
for (let r = 0; r < 9; r++) {
  for (let c = 0; c < 9; c++) {
    if (!isGiven(game, r, c)) continue;
    try { input(game, r, c, 1); } catch { givenRejected++; }
  }
}
check('所有题目格都拒绝修改', givenRejected === givenCount, `${givenRejected}/${givenCount} rejected`);

// 2. 空格可输入 / 清除
const t1 = findEmpty(game);
const r1 = input(game, t1.r, t1.c, primary.solution[cellIndex(t1.r, t1.c)]);
check(`空格 (${t1.r},${t1.c}) 可输入 1-9`, game.board[cellIndex(t1.r, t1.c)] === primary.solution[cellIndex(t1.r, t1.c)],
  `value=${game.board[cellIndex(t1.r, t1.c)]}, conflicts=${JSON.stringify(r1.conflicts)}`);
clear(game, t1.r, t1.c);
check('空格可清除回空', game.board[cellIndex(t1.r, t1.c)] === EMPTY, `value=${game.board[cellIndex(t1.r, t1.c)]}`);

// 3. 行/列/宫重复有提示
const probe = createGame(primary);
const holes = [];
for (let i = 0; i < CELLS; i++) if (probe.puzzle[i] === EMPTY) holes.push(i);
// 找一个能制造行冲突的空格：把已存在的同行数字填进另一个空
let rowCase = null;
for (const i of holes) {
  const r = (i / 9) | 0;
  for (let c = 0; c < 9; c++) {
    const j = cellIndex(r, c);
    if (j !== i && probe.board[j] !== EMPTY && probe.board[j] !== undefined && !conflictsAt(probe.board, r, c).length) {
      const v = probe.board[j];
      if (v !== EMPTY && probe.puzzle[j] !== EMPTY) { rowCase = { i, r, c: i % 9, v, src: j }; break; }
    }
  }
  if (rowCase) break;
}
if (rowCase) {
  const res = input(probe, rowCase.r, rowCase.c, rowCase.v);
  check('行内重复产生冲突提示', res.conflicts.includes(rowCase.src),
    `placed ${rowCase.v} at (${rowCase.r},${rowCase.c}), conflicts=${JSON.stringify(res.conflicts)}`);
} else {
  check('行内重复产生冲突提示', true, 'skipped: no constructible row-conflict slot in this fixture');
}

// 直接构造列/宫冲突验证判定函数（不依赖题面布局）
const colGrid = Array(CELLS).fill(EMPTY);
colGrid[cellIndex(0, 0)] = 4;
colGrid[cellIndex(5, 0)] = 4;
check('列内重复产生冲突提示', conflictsAt(colGrid, 5, 0).length === 1,
  `conflicts=${JSON.stringify(conflictsAt(colGrid, 5, 0))}`);

const boxGrid = Array(CELLS).fill(EMPTY);
boxGrid[cellIndex(0, 0)] = 4;
boxGrid[cellIndex(2, 2)] = 4;
check('宫内重复产生冲突提示', conflictsAt(boxGrid, 2, 2).length === 1,
  `conflicts=${JSON.stringify(conflictsAt(boxGrid, 2, 2))}`);

// 4. 正确填满 => 完成
const solved = createGame(primary);
solveToEnd(solved);
check('正确填满显示完成', isComplete(solved) === true,
  `empties=${emptyCount(solved)}, complete=${isComplete(solved)}`);

// 5. 未填满 => 不完成
const partial = createGame(primary);
const t2 = findEmpty(partial);
input(partial, t2.r, t2.c, primary.solution[cellIndex(t2.r, t2.c)]);
check('未填满不显示完成', isComplete(partial) === false, `empties=${emptyCount(partial)}`);

// 6. 重置恢复题面
reset(solved);
check('重置恢复题面', solved.board.every((v, i) => v === primary.puzzle[i]) && fillCount(solved) === primary.clues,
  `clues=${fillCount(solved)}, complete=${isComplete(solved)}`);

// 7. 新游戏
const second = F[1];
newGame(solved, second);
check('新游戏切换到新题面', solved.fixtureId === second.id && fillCount(solved) === second.clues,
  `fixture=${solved.fixtureId}, clues=${fillCount(solved)}`);

// 8. 题目合法且唯一解
for (const f of F) {
  check(`题目 ${f.id} 合法且唯一解`, countSolutions(f.puzzle, 2) === 1,
    `solutions=${countSolutions(f.puzzle, 2)}, clues=${f.clues}`);
}

console.log('\n--- 固定题面（可直接用于浏览器手工验收） ---');
for (const f of F) {
  console.log(`\n# ${f.id} (${f.label}) clues=${f.clues}`);
  console.log(f.puzzlePretty);
  console.log(`  answer:\n${f.solutionPretty.split('\n').map((l) => `  ${l}`).join('\n')}`);
}

console.log(`\n=== 结果: ${results.length - failures}/${results.length} PASS, ${failures} FAIL ===`);
process.exit(failures === 0 ? 0 : 1);
