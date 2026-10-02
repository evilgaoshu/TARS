#!/usr/bin/env python3
"""
EVI-142 独立复核：用与 JS 实现完全不同的代码路径（Python + 经典回溯 + set 判定）
重新验证 fixtures.json 的三个固定题面。

目的：避免「唯一解」结论只来自单一实现的盲点。
运行：python3 examples/sudoku/verify_fixtures.py
"""
import json
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent


def parse(s):
    return [int(ch) for ch in s]


def units_ok(grid):
    """行/列/宫无重复，返回冲突描述列表。"""
    bad = []
    for r in range(9):
        vals = [grid[r * 9 + c] for c in range(9) if grid[r * 9 + c]]
        if len(vals) != len(set(vals)):
            bad.append(f"row {r}")
    for c in range(9):
        vals = [grid[r * 9 + c] for r in range(9) if grid[r * 9 + c]]
        if len(vals) != len(set(vals)):
            bad.append(f"col {c}")
    for br in range(3):
        for bc in range(3):
            vals = [
                grid[(br * 3 + dr) * 9 + (bc * 3 + dc)]
                for dr in range(3)
                for dc in range(3)
                if grid[(br * 3 + dr) * 9 + (bc * 3 + dc)]
            ]
            if len(vals) != len(set(vals)):
                bad.append(f"box {br},{bc}")
    return bad


def count_solutions(grid, limit=2):
    """独立实现：按行列宫 set 剪枝的递归回溯。"""
    grid = list(grid)
    rows = [set() for _ in range(9)]
    cols = [set() for _ in range(9)]
    boxes = [set() for _ in range(9)]
    for i, v in enumerate(grid):
        if not v:
            continue
        r, c, b = i // 9, i % 9, (i // 9 // 3) * 3 + (i % 9) // 3
        if v in rows[r] or v in cols[c] or v in boxes[b]:
            return 0
        rows[r].add(v)
        cols[c].add(v)
        boxes[b].add(v)

    count = 0

    def dfs():
        nonlocal count
        if count >= limit:
            return
        best, best_cands = -1, None
        for i, v in enumerate(grid):
            if v:
                continue
            r, c, b = i // 9, i % 9, (i // 9 // 3) * 3 + (i % 9) // 3
            cands = [d for d in range(1, 10) if d not in rows[r] and d not in cols[c] and d not in boxes[b]]
            if not cands:
                return
            if best_cands is None or len(cands) < len(best_cands):
                best, best_cands = i, cands
                if len(cands) == 1:
                    break
        if best == -1:
            count += 1
            return
        r, c, b = best // 9, best % 9, (best // 9 // 3) * 3 + (best % 9) // 3
        for d in best_cands:
            grid[best] = d
            rows[r].add(d)
            cols[c].add(d)
            boxes[b].add(d)
            dfs()
            grid[best] = 0
            rows[r].discard(d)
            cols[c].discard(d)
            boxes[b].discard(d)
            if count >= limit:
                return

    dfs()
    return count


def main():
    doc = json.loads((HERE / "fixtures.json").read_text(encoding="utf-8"))
    failures = []
    for f in doc["fixtures"]:
        fid = f["id"]
        puzzle = parse(f["puzzle"])
        solution = parse(f["solution"])
        checks = []

        checks.append(("puzzle 81 cells", len(puzzle) == 81))
        checks.append(("solution 81 cells", len(solution) == 81))
        checks.append(("puzzle legal (row/col/box)", not units_ok(puzzle)))
        checks.append(("solution legal & full", not units_ok(solution) and all(1 <= v <= 9 for v in solution)))
        checks.append((
            "puzzle consistent with solution",
            all(p == 0 or p == s for p, s in zip(puzzle, solution)),
        ))
        n = count_solutions(puzzle, 2)
        checks.append((f"unique solution (found {n})", n == 1))
        checks.append(("clue count matches", sum(1 for v in puzzle if v) == f["clues"]))
        checks.append(("pretty text matches grid", f["puzzlePretty"] == "\n".join(
            "".join("." if puzzle[r * 9 + c] == 0 else str(puzzle[r * 9 + c]) for c in range(9)) for r in range(9)
        )))

        for name, ok in checks:
            print(f"  [{'PASS' if ok else 'FAIL'}] {fid}: {name}")
        if not all(ok for _, ok in checks):
            failures.append(fid)

    print()
    if failures:
        print(f"INDEPENDENT VERIFY FAILED: {failures}")
        return 1
    print(f"INDEPENDENT VERIFY OK: {len(doc['fixtures'])}/{len(doc['fixtures'])} fixtures "
          f"re-confirmed by Python solver (legal + exactly one solution)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
