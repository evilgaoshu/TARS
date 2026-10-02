/**
 * EVI-142 参考实现：游戏状态机。
 *
 * 主 DEV 的 UI 只要满足这里的语义，sudoku.conformance.test.mjs 的用例即可直接复用：
 *   - 题目格(given)不可改：input/clear 对 given 抛错且不改变棋盘
 *   - 空格可输入 1..9、可清除
 *   - 每次输入返回该格的冲突提示（行/列/宫重复的其它格）
 *   - 正确填满 => isComplete() === true；未填满 => false
 *   - reset() 恢复题面，given 不变，非 given 全部清空
 *   - newGame() 切换到新题面/新答案
 */

import {
  CELLS,
  EMPTY,
  cellIndex,
  conflictsAt,
  countSolutions,
  isLegalGrid,
  isValidFull,
  isDigit,
} from './rules.mjs';

/** 校验 fixture 合法且唯一解；不通过直接抛错，避免带着坏题目进入 UI。 */
export function assertPlayableFixture({ puzzle, solution }) {
  if (puzzle.length !== CELLS || solution.length !== CELLS) throw new Error('fixture grids must be 81 cells');
  if (!isLegalGrid(puzzle)) throw new Error('fixture puzzle is illegal (duplicate in a unit)');
  for (let i = 0; i < CELLS; i++) {
    if (puzzle[i] !== EMPTY && puzzle[i] !== solution[i]) throw new Error(`fixture puzzle cell ${i} disagrees with solution`);
  }
  if (!isValidFull(solution)) throw new Error('fixture solution is not a valid full grid');
  if (countSolutions(puzzle, 2) !== 1) throw new Error('fixture puzzle does not have exactly one solution');
}

export function createGame(fixture) {
  assertPlayableFixture(fixture);
  return {
    fixtureId: fixture.id,
    puzzle: Array.from(fixture.puzzle),
    solution: Array.from(fixture.solution),
    board: Array.from(fixture.puzzle),
  };
}

/** 题目格：初始题面里非 0 的格子，UI 必须渲染为只读。 */
export function isGiven(game, r, c) { return game.puzzle[cellIndex(r, c)] !== EMPTY; }

export function input(game, r, c, v) {
  if (!isDigit(v)) throw new RangeError(`input value must be an integer 1..9, got ${v}`);
  const i = cellIndex(r, c);
  if (game.puzzle[i] !== EMPTY) throw new Error(`cell (${r},${c}) is a given and cannot be modified`);
  game.board[i] = v;
  return { conflicts: conflictsAt(game.board, r, c), complete: isComplete(game) };
}

export function clear(game, r, c) {
  const i = cellIndex(r, c);
  if (game.puzzle[i] !== EMPTY) throw new Error(`cell (${r},${c}) is a given and cannot be cleared`);
  game.board[i] = EMPTY;
  return { conflicts: [], complete: false };
}

/** 重置：恢复题面。返回恢复后的棋盘副本。 */
export function reset(game) {
  game.board = Array.from(game.puzzle);
  return Array.from(game.board);
}

/** 新游戏：换题面与答案，given 集合随之改变。 */
export function newGame(game, fixture) {
  assertPlayableFixture(fixture);
  game.fixtureId = fixture.id;
  game.puzzle = Array.from(fixture.puzzle);
  game.solution = Array.from(fixture.solution);
  game.board = Array.from(fixture.puzzle);
  return game;
}

/** 正确填满：每格 1..9、无冲突。 */
export function isComplete(game) { return isValidFull(game.board); }

/** 与答案完全一致（更强，用于测试「正确填满显示完成」）。 */
export function isSolved(game) {
  return game.board.every((v, i) => v === game.solution[i]);
}

export function fillCount(game) { return game.board.filter((v) => v !== EMPTY).length; }

export function emptyCount(game) { return game.board.filter((v) => v === EMPTY).length; }

/** 按答案自动填满所有空格（测试用）。 */
export function solveToEnd(game) {
  for (let i = 0; i < CELLS; i++) {
    if (game.board[i] === EMPTY) game.board[i] = game.solution[i];
  }
  return game.board;
}

export function findEmpty(game) {
  const i = game.board.indexOf(EMPTY);
  return i === -1 ? null : { r: (i / 9) | 0, c: i % 9 };
}
