/**
 * EVI-142 参考实现：数独规则判定 + 游戏状态机 + 求解/唯一解计数。
 *
 * 用途：协作 DEV(DEV-PI) 提供给主 DEV(DEV-mac-opencode) 的「行为契约」与可复现测试基线。
 * 主 DEV 可以自定 UI/框架，但下列语义应与本文件一致，测试用例才可直接复用。
 *
 * 网格表示：长度 81 的普通数组，索引 i = r*9+c，值 0 表示空格，1..9 表示已填数字。
 */

export const SIZE = 9;
export const BOX = 3;
export const CELLS = 81;
export const EMPTY = 0;
export const ALL_DIGITS_MASK = 0b111111111;

export function rowOf(i) { return (i / SIZE) | 0; }
export function colOf(i) { return i % SIZE; }
export function boxOf(i) { return (((i / SIZE) | 0) / BOX | 0) * BOX + ((i % SIZE) / BOX | 0); }
export function cellIndex(r, c) { return r * SIZE + c; }

/** 27 个单元：9 行 + 9 列 + 9 宫。冲突判定只依赖它。 */
export const UNITS = (() => {
  const units = [];
  for (let r = 0; r < SIZE; r++) {
    units.push({ kind: 'row', index: r, cells: Array.from({ length: SIZE }, (_, c) => cellIndex(r, c)) });
  }
  for (let c = 0; c < SIZE; c++) {
    units.push({ kind: 'col', index: c, cells: Array.from({ length: SIZE }, (_, r) => cellIndex(r, c)) });
  }
  for (let b = 0; b < SIZE; b++) {
    const r0 = ((b / BOX) | 0) * BOX;
    const c0 = (b % BOX) * BOX;
    const cells = [];
    for (let dr = 0; dr < BOX; dr++) {
      for (let dc = 0; dc < BOX; dc++) cells.push(cellIndex(r0 + dr, c0 + dc));
    }
    units.push({ kind: 'box', index: b, cells });
  }
  return units;
})();

export function isDigit(v) { return Number.isInteger(v) && v >= 1 && v <= 9; }

/** 接受 "530070000..."、"530 070 000" 或带 '.' 的写法。 */
export function parseGrid(text) {
  const cleaned = String(text).replace(/[^0-9.]/g, '').replace(/\./g, '0');
  if (cleaned.length !== CELLS) {
    throw new Error(`grid must contain exactly ${CELLS} cells, got ${cleaned.length}`);
  }
  return Array.from(cleaned, (ch) => Number(ch));
}

/** 规范化输出：9 行，空格用 '.'。用于 fixture 摘要与截图对照。 */
export function formatGrid(grid) {
  const lines = [];
  for (let r = 0; r < SIZE; r++) {
    let line = '';
    for (let c = 0; c < SIZE; c++) line += grid[cellIndex(r, c)] === EMPTY ? '.' : String(grid[cellIndex(r, c)]);
    lines.push(line);
  }
  return lines.join('\n');
}

/** 返回所有重复项：{ kind, index, digit, cells }。空单元不参与。 */
export function unitDuplicates(grid) {
  const found = [];
  for (const unit of UNITS) {
    const byDigit = new Map();
    for (const i of unit.cells) {
      const v = grid[i];
      if (!isDigit(v)) continue;
      if (!byDigit.has(v)) byDigit.set(v, []);
      byDigit.get(v).push(i);
    }
    for (const [digit, cells] of byDigit) {
      if (cells.length > 1) found.push({ kind: unit.kind, index: unit.index, digit, cells });
    }
  }
  return found;
}

/** 题目/棋盘是否合法：任一单元内无重复数字。 */
export function isLegalGrid(grid) { return unitDuplicates(grid).length === 0; }

/** 需要高亮的全部冲突格（返回索引 Set）。 */
export function conflictCells(grid) {
  const out = new Set();
  for (const dup of unitDuplicates(grid)) for (const i of dup.cells) out.add(i);
  return out;
}

/**
 * 针对 (r,c) 这一格，返回与它同单元且同数字的其它格索引（升序）。
 * 空格返回 []；这是 UI「行/列/宫重复提示」的直接依据。
 */
export function conflictsAt(grid, r, c) {
  const i = cellIndex(r, c);
  const v = grid[i];
  if (!isDigit(v)) return [];
  const out = new Set();
  for (const unit of UNITS) {
    if (!unit.cells.includes(i)) continue;
    for (const j of unit.cells) if (j !== i && grid[j] === v) out.add(j);
  }
  return [...out].sort((a, b) => a - b);
}

/** 在 (r,c) 放 v 是否与同单元已有数字冲突（空格的占位判定，不修改入参）。 */
export function isLegalPlacement(grid, r, c, v) {
  if (!isDigit(v)) return false;
  const i = cellIndex(r, c);
  for (const unit of UNITS) {
    if (!unit.cells.includes(i)) continue;
    for (const j of unit.cells) if (j !== i && grid[j] === v) return false;
  }
  return true;
}

/** 棋盘是否「正确填满」：每格 1..9 且 27 个单元各含 1..9 恰好一次。 */
export function isValidFull(grid) {
  for (let i = 0; i < CELLS; i++) if (!isDigit(grid[i])) return false;
  return isLegalGrid(grid);
}

function popcount(x) {
  x -= (x >> 1) & 0x55555555;
  x = (x & 0x33333333) + ((x >> 2) & 0x33333333);
  x = (x + (x >> 4)) & 0x0f0f0f0f;
  return (x * 0x01010101) >> 24;
}

/**
 * 计数解的数量，最多数到 limit（默认 2）。
 * limit=2 时返回 1 即证明唯一解，返回 2 即证明不唯一。
 * 位掩码 + MRV，够快，可在测试里对每个 fixture 直接调用。
 */
export function countSolutions(grid, limit = 2) {
  if (limit < 1) throw new RangeError('limit must be >= 1');
  const cells = Int32Array.from(grid);
  const rows = new Int32Array(SIZE);
  const cols = new Int32Array(SIZE);
  const boxes = new Int32Array(SIZE);
  for (let i = 0; i < CELLS; i++) {
    const v = cells[i];
    if (v === EMPTY) continue;
    if (!isDigit(v)) throw new RangeError(`cell ${i} has invalid value ${v}`);
    const bit = 1 << (v - 1);
    if (rows[rowOf(i)] & bit || cols[colOf(i)] & bit || boxes[boxOf(i)] & bit) return 0;
    rows[rowOf(i)] |= bit;
    cols[colOf(i)] |= bit;
    boxes[boxOf(i)] |= bit;
  }

  let count = 0;
  const dfs = () => {
    if (count >= limit) return;
    let best = -1;
    let bestMask = 0;
    let bestN = 10;
    for (let i = 0; i < CELLS; i++) {
      if (cells[i] !== EMPTY) continue;
      const used = rows[rowOf(i)] | cols[colOf(i)] | boxes[boxOf(i)];
      const mask = ALL_DIGITS_MASK & ~used;
      const n = popcount(mask);
      if (n === 0) return;
      if (n < bestN) { bestN = n; best = i; bestMask = mask; if (n === 1) break; }
    }
    if (best === -1) { count++; return; }
    const r = rowOf(best);
    const c = colOf(best);
    const b = boxOf(best);
    let m = bestMask;
    while (m) {
      const bit = m & -m;
      m ^= bit;
      cells[best] = 32 - Math.clz32(bit);
      rows[r] |= bit; cols[c] |= bit; boxes[b] |= bit;
      dfs();
      cells[best] = EMPTY;
      rows[r] ^= bit; cols[c] ^= bit; boxes[b] ^= bit;
      if (count >= limit) return;
    }
  };

  dfs();
  return count;
}

/** 返回第一个解（81 长数组）或 null。 */
export function solve(grid) {
  const work = Array.from(grid);
  const rows = new Int32Array(SIZE);
  const cols = new Int32Array(SIZE);
  const boxes = new Int32Array(SIZE);
  for (let i = 0; i < CELLS; i++) {
    const v = work[i];
    if (v === EMPTY) continue;
    if (!isDigit(v)) throw new RangeError(`cell ${i} has invalid value ${v}`);
    const bit = 1 << (v - 1);
    if (rows[rowOf(i)] & bit || cols[colOf(i)] & bit || boxes[boxOf(i)] & bit) return null;
    rows[rowOf(i)] |= bit; cols[colOf(i)] |= bit; boxes[boxOf(i)] |= bit;
  }
  const dfs = () => {
    let best = -1;
    let bestMask = 0;
    let bestN = 10;
    for (let i = 0; i < CELLS; i++) {
      if (work[i] !== EMPTY) continue;
      const mask = ALL_DIGITS_MASK & ~(rows[rowOf(i)] | cols[colOf(i)] | boxes[boxOf(i)]);
      const n = popcount(mask);
      if (n === 0) return false;
      if (n < bestN) { bestN = n; best = i; bestMask = mask; if (n === 1) break; }
    }
    if (best === -1) return true;
    const r = rowOf(best);
    const c = colOf(best);
    const b = boxOf(best);
    let m = bestMask;
    while (m) {
      const bit = m & -m;
      m ^= bit;
      work[best] = 32 - Math.clz32(bit);
      rows[r] |= bit; cols[c] |= bit; boxes[b] |= bit;
      if (dfs()) return true;
      work[best] = EMPTY;
      rows[r] ^= bit; cols[c] ^= bit; boxes[b] ^= bit;
    }
    return false;
  };
  return dfs() ? work : null;
}

// ---------------------------------------------------------------------------
// 确定性生成（供 fixture 复现；不使用 Math.random）
// ---------------------------------------------------------------------------

/** mulberry32：同 seed 必得同序列。 */
export function mulberry32(seed) {
  let a = seed >>> 0;
  return function next() {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function shuffle(arr, rand) {
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(rand() * (i + 1));
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
}

/** 生成一个完整合法终盘。 */
export function generateSolution(seed) {
  const rand = mulberry32(seed);
  const grid = new Array(CELLS).fill(EMPTY);
  const rows = new Int32Array(SIZE);
  const cols = new Int32Array(SIZE);
  const boxes = new Int32Array(SIZE);
  const fill = (i) => {
    if (i === CELLS) return true;
    const r = rowOf(i);
    const c = colOf(i);
    const b = boxOf(i);
    const mask = ALL_DIGITS_MASK & ~(rows[r] | cols[c] | boxes[b]);
    const cands = [];
    for (let v = 1; v <= SIZE; v++) if (mask & (1 << (v - 1))) cands.push(v);
    shuffle(cands, rand);
    for (const v of cands) {
      const bit = 1 << (v - 1);
      grid[i] = v; rows[r] |= bit; cols[c] |= bit; boxes[b] |= bit;
      if (fill(i + 1)) return true;
      grid[i] = EMPTY; rows[r] ^= bit; cols[c] ^= bit; boxes[b] ^= bit;
    }
    return false;
  };
  if (!fill(0)) throw new Error('failed to generate a full grid');
  return grid;
}

/**
 * 从终盘挖空得到唯一解题目：按随机顺序逐个挖，每次挖完立刻用 countSolutions(...,2)
 * 复核，破坏唯一性就立刻回填。因此返回值保证「合法 + 唯一解」。
 */
export function generatePuzzle(solution, seed, targetClues = 34) {
  const rand = mulberry32(seed);
  const puzzle = Array.from(solution);
  let clues = CELLS;
  for (const i of shuffle(Array.from({ length: CELLS }, (_, k) => k), rand)) {
    if (clues <= targetClues) break;
    const saved = puzzle[i];
    puzzle[i] = EMPTY;
    if (countSolutions(puzzle, 2) === 1) clues--;
    else puzzle[i] = saved;
  }
  return puzzle;
}
