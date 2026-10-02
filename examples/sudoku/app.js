/**
 * EVI-142 数独前端：examples/sudoku/index.html 的交互层。
 *
 * 只做渲染与输入路由，规则判定全部走 rules.mjs / game.mjs（与 node --test 用例同一份实现）。
 * 自动化钩子：window.__sudoku 暴露只读状态与可编程操作，供 DEV/QA 用浏览器脚本取证。
 */

import { CELLS, EMPTY, cellIndex, conflictCells, conflictsAt, isDigit, parseGrid } from './rules.mjs';
import {
  assertPlayableFixture,
  clear as clearCell,
  createGame,
  emptyCount,
  fillCount,
  input as inputCell,
  isComplete,
  isGiven,
  newGame as switchGame,
  reset as resetGame,
} from './game.mjs';

const $ = (sel) => document.querySelector(sel);
const boardEl = $('#board');
const statusEl = $('#status');
const timerEl = $('#timer');
const fixtureLabelEl = $('#fixture-label');
const fixtureSelect = $('#fixture-select');
const bannerEl = $('#banner');
const bannerTitle = $('#banner-title');
const bannerDetail = $('#banner-detail');

const pad2 = (n) => String(n).padStart(2, '0');

async function loadFixtures() {
  const res = await fetch('./fixtures.json');
  if (!res.ok) throw new Error(`fixtures.json: HTTP ${res.status}`);
  const doc = await res.json();
  return doc.fixtures.map((f) => ({
    ...f,
    puzzle: parseGrid(f.puzzle),
    solution: parseGrid(f.solution),
  }));
}

const fixtures = await loadFixtures();
fixtures.forEach((f) => assertPlayableFixture(f));

let game = createGame(fixtures[0]);
let fixtureIndex = 0;
let selected = null;
let elapsed = 0;
let timerHandle = null;
let started = false;

// ─── 计时 ────────────────────────────────────────────────────────────────────
function renderTimer() {
  timerEl.textContent = `${pad2(Math.floor(elapsed / 60))}:${pad2(elapsed % 60)}`;
}

function startTimer() {
  if (timerHandle) return;
  started = true;
  timerHandle = setInterval(() => {
    elapsed += 1;
    renderTimer();
  }, 1000);
}

function stopTimer() {
  if (timerHandle) {
    clearInterval(timerHandle);
    timerHandle = null;
  }
}

function resetTimer() {
  stopTimer();
  elapsed = 0;
  started = false;
  renderTimer();
}

// ─── 状态条 ──────────────────────────────────────────────────────────────────
function setStatus(text, state) {
  statusEl.textContent = text;
  statusEl.dataset.state = state;
}

// ─── 棋盘渲染 ────────────────────────────────────────────────────────────────
const cells = [];

function buildBoard() {
  const frag = document.createDocumentFragment();
  for (let r = 0; r < 9; r++) {
    for (let c = 0; c < 9; c++) {
      const i = cellIndex(r, c);
      const input = document.createElement('input');
      input.type = 'text';
      input.inputMode = 'numeric';
      input.autocomplete = 'off';
      input.maxLength = 1;
      input.className = 'cell';
      input.dataset.row = String(r);
      input.dataset.col = String(c);
      input.dataset.index = String(i);
      input.setAttribute('role', 'gridcell');
      input.setAttribute('aria-label', `第 ${r + 1} 行 第 ${c + 1} 列`);
      frag.appendChild(input);
      cells[i] = input;
    }
  }
  boardEl.appendChild(frag);
}

function buildNumpad() {
  const numpad = $('#numpad');
  for (let d = 1; d <= 9; d++) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'num';
    btn.dataset.digit = String(d);
    btn.textContent = String(d);
    btn.setAttribute('aria-label', `填写 ${d}`);
    numpad.appendChild(btn);
  }
}

function buildFixtureSelect() {
  fixtures.forEach((f, idx) => {
    const opt = document.createElement('option');
    opt.value = String(idx);
    opt.textContent = f.label;
    fixtureSelect.appendChild(opt);
  });
}

function render() {
  const conflictIdx = conflictCells(game.board);
  for (let i = 0; i < CELLS; i++) {
    const el = cells[i];
    const v = game.board[i];
    const given = isGiven(game, (i / 9) | 0, i % 9);
    if (el.value !== (v === EMPTY ? '' : String(v))) el.value = v === EMPTY ? '' : String(v);
    el.classList.toggle('given', given);
    el.classList.toggle('user', !given && v !== EMPTY);
    el.classList.toggle('conflict', conflictIdx.has(i));
    el.readOnly = given;
    el.tabIndex = selected === null ? (i === firstFocusable() ? 0 : -1) : i === selected ? 0 : -1;
  }
  updatePeerHighlight();
}

function firstFocusable() {
  const empty = game.board.indexOf(EMPTY);
  return empty === -1 ? 0 : empty;
}

function updatePeerHighlight() {
  for (let i = 0; i < CELLS; i++) cells[i].classList.remove('peer', 'selected');
  if (selected === null) return;
  cells[selected].classList.add('selected');
  const r = (selected / 9) | 0;
  const c = selected % 9;
  const boxR = (r / 3) | 0;
  const boxC = (c / 3) | 0;
  for (let j = 0; j < CELLS; j++) {
    const jr = (j / 9) | 0;
    const jc = j % 9;
    const samePeer =
      jr === r || jc === c || (((jr / 3) | 0) === boxR && ((jc / 3) | 0) === boxC);
    if (samePeer && j !== selected) cells[j].classList.add('peer');
  }
}

function select(index) {
  selected = index;
  updatePeerHighlight();
  if (index !== null) cells[index].focus({ preventScroll: true });
}

// ─── 输入路由 ────────────────────────────────────────────────────────────────
function applyDigit(r, c, digit) {
  const i = cellIndex(r, c);
  if (isGiven(game, r, c)) {
    setStatus(`第 ${r + 1} 行第 ${c + 1} 列是题目格，不可修改`, 'warn');
    render();
    return false;
  }
  try {
    const result = inputCell(game, r, c, digit);
    if (!started) startTimer();
    render();
    if (result.conflicts.length > 0) {
      setStatus(`第 ${r + 1} 行第 ${c + 1} 列的 ${digit} 与行/列/宫重复`, 'warn');
    } else if (isComplete(game)) {
      finish();
    } else {
      setStatus(`已填入 ${digit} · 还剩 ${emptyCount(game)} 格`, 'ok');
    }
    return true;
  } catch (err) {
    setStatus(err.message, 'warn');
    render();
    return false;
  }
}

function applyClear(r, c) {
  if (isGiven(game, r, c)) {
    setStatus(`第 ${r + 1} 行第 ${c + 1} 列是题目格，不可清除`, 'warn');
    render();
    return false;
  }
  try {
    clearCell(game, r, c);
    hideBanner();
    render();
    setStatus(`已清除第 ${r + 1} 行第 ${c + 1} 列`, 'idle');
    return true;
  } catch (err) {
    setStatus(err.message, 'warn');
    render();
    return false;
  }
}

function finish() {
  stopTimer();
  setStatus('已完成', 'done');
  bannerTitle.textContent = '完成';
  bannerDetail.textContent = `${fixtureLabelEl.textContent} · 用时 ${timerEl.textContent} · 81/81 格正确填满`;
  bannerEl.hidden = false;
}

function hideBanner() {
  bannerEl.hidden = true;
}

// ─── 事件绑定 ────────────────────────────────────────────────────────────────
boardEl.addEventListener('focusin', (e) => {
  const el = e.target;
  if (!(el instanceof HTMLInputElement)) return;
  const r = Number(el.dataset.row);
  const c = Number(el.dataset.col);
  selected = Number(el.dataset.index);
  updatePeerHighlight();
  if (isGiven(game, r, c)) {
    setStatus(`第 ${r + 1} 行第 ${c + 1} 列是题目格，不可修改`, 'warn');
  }
});

boardEl.addEventListener('click', (e) => {
  const el = e.target;
  if (!(el instanceof HTMLInputElement)) return;
  select(Number(el.dataset.index));
});

boardEl.addEventListener('input', (e) => {
  const el = e.target;
  if (!(el instanceof HTMLInputElement)) return;
  const r = Number(el.dataset.row);
  const c = Number(el.dataset.col);
  const cleaned = el.value.replace(/[^1-9]/g, '');
  el.value = cleaned.length > 0 ? cleaned.slice(-1) : '';
  if (el.value === '') {
    if (game.board[cellIndex(r, c)] !== EMPTY) applyClear(r, c);
    return;
  }
  applyDigit(r, c, Number(el.value));
});

boardEl.addEventListener('keydown', (e) => {
  const el = e.target;
  if (!(el instanceof HTMLInputElement)) return;
  const r = Number(el.dataset.row);
  const c = Number(el.dataset.col);
  const moves = {
    ArrowUp: [-1, 0],
    ArrowDown: [1, 0],
    ArrowLeft: [0, -1],
    ArrowRight: [0, 1],
  };
  if (moves[e.key]) {
    e.preventDefault();
    const nr = Math.min(8, Math.max(0, r + moves[e.key][0]));
    const nc = Math.min(8, Math.max(0, c + moves[e.key][1]));
    select(cellIndex(nr, nc));
    return;
  }
  if (e.key === 'Backspace' || e.key === 'Delete') {
    e.preventDefault();
    applyClear(r, c);
    return;
  }
  if (/^[1-9]$/.test(e.key)) {
    e.preventDefault();
    applyDigit(r, c, Number(e.key));
  }
});

$('#numpad').addEventListener('click', (e) => {
  const btn = e.target;
  if (!(btn instanceof HTMLButtonElement)) return;
  let target = selected;
  if (target === null || isGiven(game, (target / 9) | 0, target % 9)) {
    target = game.board.indexOf(EMPTY);
    if (target === -1) {
      setStatus('棋盘已填满', 'ok');
      return;
    }
    select(target);
  }
  applyDigit((target / 9) | 0, target % 9, Number(btn.dataset.digit));
});

$('#btn-clear').addEventListener('click', () => {
  if (selected === null) {
    setStatus('先选中一个格子', 'idle');
    return;
  }
  applyClear((selected / 9) | 0, selected % 9);
});

$('#btn-reset').addEventListener('click', () => {
  resetGame(game);
  resetTimer();
  hideBanner();
  render();
  setStatus('已重置为题目初始状态', 'idle');
});

function loadFixture(index, message) {
  fixtureIndex = index;
  switchGame(game, fixtures[index]);
  fixtureSelect.value = String(index);
  fixtureLabelEl.textContent = `${fixtures[index].label} · ${fixtures[index].id}`;
  selected = null;
  resetTimer();
  hideBanner();
  render();
  setStatus(message, 'idle');
}

$('#btn-new').addEventListener('click', () => {
  loadFixture((fixtureIndex + 1) % fixtures.length, '新游戏已开始');
});

fixtureSelect.addEventListener('change', () => {
  loadFixture(Number(fixtureSelect.value), '已切换题面');
});

// ─── 自动化钩子（DEV/QA 取证用；不参与渲染逻辑） ──────────────────────────────
window.__sudoku = {
  get state() {
    const conflictIdx = [...conflictCells(game.board)];
    return {
      fixtureId: game.fixtureId,
      fixtureIndex,
      board: [...game.board],
      puzzle: [...game.puzzle],
      givens: fillCount({ board: game.puzzle }),
      empties: emptyCount(game),
      complete: isComplete(game),
      started,
      elapsed,
      selected,
      conflicts: conflictIdx,
      status: statusEl.textContent,
      bannerHidden: bannerEl.hidden,
    };
  },
  select: (r, c) => select(cellIndex(r, c)),
  input: (r, c, v) => applyDigit(r, c, v),
  clear: (r, c) => applyClear(r, c),
  conflictsAt: (r, c) => conflictsAt(game.board, r, c),
  reset: () => $('#btn-reset').click(),
  newGame: () => $('#btn-new').click(),
  loadFixture: (id) => {
    const idx = fixtures.findIndex((f) => f.id === id);
    if (idx === -1) throw new Error(`unknown fixture: ${id}`);
    loadFixture(idx, '已切换题面');
  },
  solveToEnd: () => {
    game.board = [...game.solution];
    render();
    if (isComplete(game)) finish();
  },
  isDigit,
};

// ─── 启动 ────────────────────────────────────────────────────────────────────
buildBoard();
buildNumpad();
buildFixtureSelect();
fixtureLabelEl.textContent = `${fixtures[0].label} · ${fixtures[0].id}`;
render();
setStatus('未开始 · 选择空格开始填写', 'idle');
