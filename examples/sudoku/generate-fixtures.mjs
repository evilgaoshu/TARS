/**
 * EVI-142 固定题面生成器（确定性）。
 *
 * 运行：node examples/sudoku/generate-fixtures.mjs
 * 产出：examples/sudoku/fixtures.json
 *
 * 每个 fixture 都带 uniqueSolution 证明（countSolutions(...,2) === 1）与 clue 数。
 * 因为 seed 固定，任何人重跑得到完全相同的题面，测试不会漂移。
 */

import { writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import {
  CELLS,
  EMPTY,
  countSolutions,
  formatGrid,
  generatePuzzle,
  generateSolution,
  isLegalGrid,
  isValidFull,
} from './rules.mjs';

const HERE = dirname(fileURLToPath(import.meta.url));

// seed 固定 => 题面固定。三题覆盖不同挖空程度。
const SPECS = [
  { id: 'easy-seed-1421', seed: 1421, targetClues: 40, label: '简单（40 提示数）' },
  { id: 'medium-seed-7331', seed: 7331, targetClues: 34, label: '中等（约 34 提示数）' },
  { id: 'hard-seed-9091', seed: 9091, targetClues: 28, label: '偏难（约 28 提示数）' },
];

const fixtures = [];
for (const spec of SPECS) {
  const solution = generateSolution(spec.seed);
  if (!isValidFull(solution)) throw new Error(`${spec.id}: generated solution is invalid`);
  const puzzle = generatePuzzle(solution, spec.seed, spec.targetClues);
  if (!isLegalGrid(puzzle)) throw new Error(`${spec.id}: generated puzzle is illegal`);
  const solutions = countSolutions(puzzle, 2);
  if (solutions !== 1) throw new Error(`${spec.id}: puzzle has ${solutions} solutions, expected exactly 1`);
  const clues = puzzle.filter((v) => v !== EMPTY).length;
  if (clues !== puzzle.filter((v) => v !== EMPTY).length) throw new Error('unreachable');
  fixtures.push({
    id: spec.id,
    label: spec.label,
    seed: spec.seed,
    targetClues: spec.targetClues,
    clues,
    empties: CELLS - clues,
    uniqueSolution: true,
    puzzle: puzzle.join(''),
    solution: solution.join(''),
    puzzlePretty: formatGrid(puzzle),
    solutionPretty: formatGrid(solution),
  });
}

const out = {
  generatedBy: 'examples/sudoku/generate-fixtures.mjs',
  generator: 'deterministic mulberry32 seeds; dig holes then re-verify uniqueness after each removal',
  fixtures,
};

writeFileSync(join(HERE, 'fixtures.json'), `${JSON.stringify(out, null, 2)}\n`, 'utf8');

for (const f of fixtures) {
  console.log(`${f.id}  clues=${f.clues}  empties=${f.empties}  unique=${f.uniqueSolution}  seed=${f.seed}`);
}
console.log(`\nwrote ${fixtures.length} fixtures -> examples/sudoku/fixtures.json`);
