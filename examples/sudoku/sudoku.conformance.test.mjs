/**
 * EVI-142 协作 DEV 交付的可复现测试用例（行/列/宫冲突、正确完成、未填满、重置、题目合法且唯一解）。
 *
 * 运行：node --test examples/sudoku/
 * 依赖：仅 Node 内置 node:test + node:assert，无需 npm install。
 *
 * 这些用例是「行为契约」，主 DEV 可以把同样的断言指向自己的 UI/状态模块；
 * 若实现通过，说明 issue 里那几条 hard requirement 的判定语义与协作 DEV 基线一致。
 */

import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

import {
  CELLS,
  EMPTY,
  cellIndex,
  conflictCells,
  conflictsAt,
  countSolutions,
  formatGrid,
  isLegalGrid,
  isLegalPlacement,
  isValidFull,
  parseGrid,
  solve,
  unitDuplicates,
} from './rules.mjs';
import {
  assertPlayableFixture,
  clear,
  createGame,
  emptyCount,
  fillCount,
  findEmpty,
  input,
  isComplete,
  isGiven,
  isSolved,
  newGame,
  reset,
  solveToEnd,
} from './game.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));
const fixturesDoc = JSON.parse(readFileSync(join(HERE, 'fixtures.json'), 'utf8'));
const FIXTURES = fixturesDoc.fixtures.map((f) => ({
  ...f,
  puzzle: parseGrid(f.puzzle),
  solution: parseGrid(f.solution),
}));

const PRIMARY = FIXTURES[0];

// ---------------------------------------------------------------------------
// 1. 固定题面自身：合法 + 唯一解
// ---------------------------------------------------------------------------

test('fixtures: 至少提供 3 个固定题面，且每个都是 81 格', () => {
  assert.ok(FIXTURES.length >= 3, `expected >= 3 fixtures, got ${FIXTURES.length}`);
  for (const f of FIXTURES) {
    assert.equal(f.puzzle.length, CELLS, `${f.id}: puzzle length`);
    assert.equal(f.solution.length, CELLS, `${f.id}: solution length`);
    assert.ok(f.clues > 0 && f.clues < CELLS, `${f.id}: clue count must be interior`);
    assert.equal(f.puzzle.filter((v) => v !== EMPTY).length, f.clues, `${f.id}: clue count matches grid`);
  }
});

for (const f of FIXTURES) {
  test(`fixture ${f.id}: 题面合法（行/列/宫无重复）`, () => {
    assert.equal(unitDuplicates(f.puzzle).length, 0, 'puzzle has duplicates');
    assert.ok(isLegalGrid(f.puzzle));
  });

  test(`fixture ${f.id}: 答案是一个合法终盘`, () => {
    assert.ok(isValidFull(f.solution), 'solution must be a valid full grid');
  });

  test(`fixture ${f.id}: 题面与答案不矛盾（每个提示数都对）`, () => {
    for (let i = 0; i < CELLS; i++) {
      if (f.puzzle[i] !== EMPTY) assert.equal(f.puzzle[i], f.solution[i], `cell ${i}`);
    }
  });

  test(`fixture ${f.id}: 题目恰好唯一解（countSolutions(...,2) === 1）`, () => {
    assert.equal(countSolutions(f.puzzle, 2), 1);
  });

  test(`fixture ${f.id}: solve() 返回的正是记录的答案`, () => {
    assert.deepEqual(solve(f.puzzle), f.solution);
  });

  test(`fixture ${f.id}: 固定题面可复现（fixtures.json 里的 pretty 文本与数组一致）`, () => {
    assert.equal(formatGrid(f.puzzle), f.puzzlePretty);
    assert.equal(formatGrid(f.solution), f.solutionPretty);
  });
}

test('唯一解检查器本身有效：能区分唯一解与多解（证明断言非恒真）', () => {
  const f = PRIMARY;
  assert.equal(countSolutions(f.puzzle, 2), 1, 'unique puzzle must report exactly 1');
  assert.equal(countSolutions(f.solution, 2), 1, 'a complete grid has exactly 1 completion');

  const emptyGrid = Array(CELLS).fill(EMPTY);
  assert.equal(countSolutions(emptyGrid, 2), 2, 'empty grid must reach the >=2 limit');

  // 只留 2 个提示数（分属不同行/列/宫）=> 必然多解
  const sparse = Array(CELLS).fill(EMPTY);
  sparse[cellIndex(0, 0)] = f.solution[cellIndex(0, 0)];
  sparse[cellIndex(4, 4)] = f.solution[cellIndex(4, 4)];
  assert.ok(isLegalGrid(sparse), 'sparse grid is legal');
  assert.equal(countSolutions(sparse, 2), 2, 'two-clue grid must reach the >=2 limit');

  // 逐格挖空扫描：至少存在一个提示数，其移除会让题目变为多解
  // （若全部移除后仍唯一，说明题目已是最小题目；两种结论都打印出来便于主 DEV 核对）
  let breakable = 0;
  for (let i = 0; i < CELLS; i++) {
    if (f.puzzle[i] === EMPTY) continue;
    const trial = Array.from(f.puzzle);
    trial[i] = EMPTY;
    if (countSolutions(trial, 2) > 1) breakable++;
  }
  assert.ok(breakable >= 1, `${f.id} should have at least one removable clue (found ${breakable})`);
});

// ---------------------------------------------------------------------------
// 2. 行 / 列 / 宫冲突
// ---------------------------------------------------------------------------

test('冲突: 行内重复被检出并指向正确两格', () => {
  const grid = Array(CELLS).fill(EMPTY);
  grid[cellIndex(0, 0)] = 5;
  grid[cellIndex(0, 8)] = 5;
  const dups = unitDuplicates(grid);
  assert.equal(dups.length, 1);
  assert.equal(dups[0].kind, 'row');
  assert.equal(dups[0].index, 0);
  assert.equal(dups[0].digit, 5);
  assert.deepEqual(dups[0].cells, [cellIndex(0, 0), cellIndex(0, 8)]);
  assert.deepEqual(conflictsAt(grid, 0, 0), [cellIndex(0, 8)]);
  assert.deepEqual(conflictsAt(grid, 0, 8), [cellIndex(0, 0)]);
});

test('冲突: 列内重复被检出', () => {
  const grid = Array(CELLS).fill(EMPTY);
  grid[cellIndex(2, 3)] = 7;
  grid[cellIndex(7, 3)] = 7;
  const dups = unitDuplicates(grid);
  assert.equal(dups.length, 1);
  assert.equal(dups[0].kind, 'col');
  assert.equal(dups[0].index, 3);
  assert.deepEqual(conflictsAt(grid, 7, 3), [cellIndex(2, 3)]);
});

test('冲突: 宫内重复被检出（且同行同列不成立，证明是宫判定）', () => {
  const grid = Array(CELLS).fill(EMPTY);
  // (1,1) 与 (2,2) 同宫不同行不同列
  grid[cellIndex(1, 1)] = 9;
  grid[cellIndex(2, 2)] = 9;
  const dups = unitDuplicates(grid);
  assert.equal(dups.length, 1);
  assert.equal(dups[0].kind, 'box');
  assert.equal(dups[0].index, 0);
  assert.deepEqual(conflictsAt(grid, 1, 1), [cellIndex(2, 2)]);
});

test('冲突: 同一格可同时触发行与列冲突（去重后返回并集）', () => {
  const grid = Array(CELLS).fill(EMPTY);
  grid[cellIndex(4, 4)] = 3;
  grid[cellIndex(4, 6)] = 3; // 同行
  grid[cellIndex(6, 4)] = 3; // 同列
  assert.deepEqual(conflictsAt(grid, 4, 4), [cellIndex(4, 6), cellIndex(6, 4)]);
  assert.deepEqual([...conflictCells(grid)].sort((a, b) => a - b), [
    cellIndex(4, 4), cellIndex(4, 6), cellIndex(6, 4),
  ]);
});

test('冲突: 空格不参与冲突判定', () => {
  const grid = Array(CELLS).fill(EMPTY);
  assert.deepEqual(conflictsAt(grid, 0, 0), []);
  assert.equal(unitDuplicates(grid).length, 0);
  assert.ok(isLegalGrid(grid));
});

test('冲突: isLegalPlacement 预检不改动棋盘且与事后判定一致', () => {
  const grid = Array(CELLS).fill(EMPTY);
  grid[cellIndex(0, 4)] = 6;
  const before = grid.join('');
  assert.equal(isLegalPlacement(grid, 0, 7, 6), false, 'row conflict');
  assert.equal(isLegalPlacement(grid, 8, 4, 6), false, 'col conflict');
  assert.equal(isLegalPlacement(grid, 2, 5, 6), false, 'box conflict');
  assert.equal(isLegalPlacement(grid, 8, 8, 6), true, 'no conflict');
  assert.equal(isLegalPlacement(grid, 0, 0, 0), false, '0 is not a digit');
  assert.equal(grid.join(''), before, 'must not mutate input');
});

// ---------------------------------------------------------------------------
// 3. 正确完成 / 未填满
// ---------------------------------------------------------------------------

test('完成: 空棋盘未填满 => isComplete() === false', () => {
  const game = createGame(PRIMARY);
  assert.equal(isComplete(game), false);
  assert.equal(fillCount(game), PRIMARY.clues);
  assert.equal(emptyCount(game), CELLS - PRIMARY.clues);
});

test('完成: 正确填满 => isComplete() === true 且与答案一致', () => {
  const game = createGame(PRIMARY);
  solveToEnd(game);
  assert.equal(emptyCount(game), 0);
  assert.equal(isComplete(game), true);
  assert.equal(isSolved(game), true);
});

test('完成: 差一格不算完成', () => {
  const game = createGame(PRIMARY);
  solveToEnd(game);
  const empty = findEmpty(game); // 无空格
  assert.equal(empty, null);
  const i = game.board.findIndex((v, idx) => game.puzzle[idx] === EMPTY);
  game.board[i] = EMPTY;
  assert.equal(isComplete(game), false, 'one hole must not count as complete');
});

test('完成: 填满但有冲突 => 不算完成', () => {
  const game = createGame(PRIMARY);
  solveToEnd(game);
  // 同一行内交换两格：棋盘仍是满的，但行内出现重复
  const board = Array.from(game.solution);
  const a = cellIndex(0, 0);
  const b = cellIndex(0, 1);
  [board[a], board[b]] = [board[b], board[a]];
  assert.equal(board.includes(EMPTY), false, 'board is full');
  assert.ok(unitDuplicates(board).length > 0, 'swap within a row must produce a duplicate');
  assert.equal(isValidFull(board), false, 'full-but-conflicting must not be valid');

  game.board = board;
  assert.equal(isComplete(game), false);
});

test('完成: 填入错误数字不会误报完成', () => {
  const game = createGame(PRIMARY);
  const target = findEmpty(game);
  const wrong = game.solution[cellIndex(target.r, target.c)] % 9 + 1;
  const res = input(game, target.r, target.c, wrong);
  assert.equal(res.complete, false);
  assert.equal(isComplete(game), false);
});

test('完成: 每次 input 都返回该格冲突格列表', () => {
  const game = createGame(PRIMARY);
  const target = findEmpty(game);
  const res = input(game, target.r, target.c, game.solution[cellIndex(target.r, target.c)]);
  assert.deepEqual(res.conflicts, conflictsAt(game.board, target.r, target.c));
  assert.deepEqual(res.conflicts, []);
  assert.equal(isGiven(game, target.r, target.c), false);
});

// ---------------------------------------------------------------------------
// 4. 重置
// ---------------------------------------------------------------------------

test('重置: 恢复题面，非题目格清空，题目格不变', () => {
  const game = createGame(PRIMARY);
  const target = findEmpty(game);
  input(game, target.r, target.c, game.solution[cellIndex(target.r, target.c)]);
  assert.ok(fillCount(game) > PRIMARY.clues, 'precondition: something was filled');

  const restored = reset(game);
  assert.deepEqual(restored, PRIMARY.puzzle);
  assert.deepEqual(game.board, PRIMARY.puzzle);
  assert.equal(fillCount(game), PRIMARY.clues);
  assert.equal(isComplete(game), false);
  assert.equal(isSolved(game), false);
  for (let r = 0; r < 9; r++) {
    for (let c = 0; c < 9; c++) {
      if (isGiven(game, r, c)) assert.equal(game.board[cellIndex(r, c)], game.puzzle[cellIndex(r, c)]);
    }
  }
});

test('重置: 填满后重置也回到题面（不是回到上一次状态）', () => {
  const game = createGame(PRIMARY);
  solveToEnd(game);
  assert.equal(isComplete(game), true);
  reset(game);
  assert.deepEqual(game.board, PRIMARY.puzzle);
  assert.equal(isComplete(game), false);
});

// ---------------------------------------------------------------------------
// 5. 题目格不可改 / 空格可输入可清除
// ---------------------------------------------------------------------------

test('题目格: input 抛错且不改变棋盘', () => {
  const game = createGame(PRIMARY);
  const givenIdx = PRIMARY.puzzle.findIndex((v) => v !== EMPTY);
  const r = (givenIdx / 9) | 0;
  const c = givenIdx % 9;
  const before = game.board.join('');
  assert.throws(() => input(game, r, c, 1), /given/);
  assert.equal(game.board.join(''), before);
});

test('题目格: clear 抛错且不改变棋盘', () => {
  const game = createGame(PRIMARY);
  const givenIdx = PRIMARY.puzzle.findIndex((v) => v !== EMPTY);
  const r = (givenIdx / 9) | 0;
  const c = givenIdx % 9;
  const before = game.board.join('');
  assert.throws(() => clear(game, r, c), /given/);
  assert.equal(game.board.join(''), before);
});

test('空格: 可输入 1..9，可清除，非法值被拒', () => {
  const game = createGame(PRIMARY);
  const t = findEmpty(game);
  for (let v = 1; v <= 9; v++) {
    input(game, t.r, t.c, v);
    assert.equal(game.board[cellIndex(t.r, t.c)], v);
  }
  assert.throws(() => input(game, t.r, t.c, 0), RangeError);
  assert.throws(() => input(game, t.r, t.c, 10), RangeError);
  assert.throws(() => input(game, t.r, t.c, 1.5), RangeError);
  const res = clear(game, t.r, t.c);
  assert.equal(game.board[cellIndex(t.r, t.c)], EMPTY);
  assert.deepEqual(res.conflicts, []);
});

// ---------------------------------------------------------------------------
// 6. 新游戏
// ---------------------------------------------------------------------------

test('新游戏: 切换题面与答案，given 集合随之改变，状态回到初始', () => {
  const game = createGame(PRIMARY);
  const second = FIXTURES[1];
  solveToEnd(game);
  newGame(game, second);
  assert.equal(game.fixtureId, second.id);
  assert.deepEqual(game.board, second.puzzle);
  assert.deepEqual(game.solution, second.solution);
  assert.equal(isComplete(game), false);
  const givenCount = game.board.filter((v, i) => game.puzzle[i] !== EMPTY).length;
  assert.equal(givenCount, second.clues);
  // 第一个题面挖空的格子，在新题面里必须是可编辑的
  const t = findEmpty(game);
  assert.equal(isGiven(game, t.r, t.c), false);
});

test('新游戏: 拒绝多解 fixture 且不改变当前局', () => {
  const game = createGame(PRIMARY);
  // 只保留两个提示数 => 必然多解，但仍是答案的合法子集
  const ambiguous = Array(CELLS).fill(EMPTY);
  ambiguous[cellIndex(0, 0)] = PRIMARY.solution[cellIndex(0, 0)];
  ambiguous[cellIndex(4, 4)] = PRIMARY.solution[cellIndex(4, 4)];
  assert.equal(countSolutions(ambiguous, 2), 2, 'precondition: ambiguous puzzle');
  assert.throws(
    () => newGame(game, { id: 'ambiguous', puzzle: ambiguous, solution: PRIMARY.solution }),
    /exactly one solution/,
  );
  assert.equal(game.fixtureId, PRIMARY.id, 'failed newGame must not change current game');
  assert.deepEqual(game.board, PRIMARY.puzzle);
});

test('新游戏: 拒绝非法题面（单元内重复）', () => {
  const game = createGame(PRIMARY);
  const illegal = Array.from(PRIMARY.puzzle);
  const givens = [];
  for (let i = 0; i < CELLS && givens.length < 2; i++) if (illegal[i] !== EMPTY) givens.push(i);
  illegal[givens[1]] = illegal[givens[0]]; // 制造重复
  assert.throws(
    () => newGame(game, { id: 'illegal', puzzle: illegal, solution: PRIMARY.solution }),
    /illegal|disagrees/,
  );
  assert.equal(game.fixtureId, PRIMARY.id);
});

test('fixture 校验: 答案与题面矛盾时被拒绝', () => {
  const bad = Array.from(PRIMARY.solution);
  bad[0] = bad[0] % 9 + 1;
  assert.throws(
    () => assertPlayableFixture({ puzzle: PRIMARY.puzzle, solution: bad }),
    /not a valid full grid|disagrees/,
  );
});

// ---------------------------------------------------------------------------
// 7. 唯一解与完成态的联合：唯一解题目的正确填满必然等于记录答案
// ---------------------------------------------------------------------------

test('联合: 唯一解题目在 UI 上只有一种「正确填满」结果', () => {
  for (const f of FIXTURES) {
    const game = createGame(f);
    solveToEnd(game);
    assert.equal(isComplete(game), true, `${f.id}: filled board must be complete`);
    assert.deepEqual(game.board, f.solution, `${f.id}: unique solution must equal recorded answer`);
  }
});

test('联合: 提示数越多空格越少，且都保持唯一解', () => {
  const sorted = [...FIXTURES].sort((a, b) => b.clues - a.clues);
  assert.equal(sorted[0].id, 'easy-seed-1421');
  for (const f of FIXTURES) assert.equal(countSolutions(f.puzzle, 2), 1);
});
