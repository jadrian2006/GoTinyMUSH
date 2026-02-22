import { useRef, useCallback } from "preact/hooks";
import { panes, gridCols, swapPanes, draggingPaneId, rowHeights } from "../stores/paneStore";
import { PaneContainer } from "./PaneContainer";

export function WindowGrid() {
  const gridRef = useRef<HTMLDivElement>(null);
  const cols = gridCols.value;
  const visiblePanes = panes.value.filter((p) => !p.poppedOut);
  const poppedOutPanes = panes.value.filter((p) => p.poppedOut);
  const dragging = draggingPaneId.value;

  // Compute number of rows needed
  const maxRow = visiblePanes.reduce(
    (max, p) => Math.max(max, p.gridRow + p.gridRowSpan),
    1,
  );

  // Build row height fr values
  const heights = rowHeights.value;
  const rowFrs: string[] = [];
  for (let r = 0; r < maxRow; r++) {
    const h = heights[r] ?? 1;
    rowFrs.push(`${h}fr`);
  }

  const handleDragOver = (e: DragEvent) => {
    e.preventDefault();
    if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  };

  const handleDrop = (e: DragEvent, targetPaneId: string) => {
    e.preventDefault();
    const sourceId = e.dataTransfer?.getData("text/plain");
    if (sourceId && sourceId !== targetPaneId) {
      swapPanes(sourceId, targetPaneId);
    }
    draggingPaneId.value = null;
  };

  // Row resize: on mousedown, track drag and adjust adjacent row heights
  const handleResizeStart = useCallback((e: MouseEvent, row: number) => {
    e.preventDefault();
    const gridEl = gridRef.current;
    if (!gridEl) return;

    // Read actual rendered row heights in px from the grid
    const computedRows = getComputedStyle(gridEl).gridTemplateRows
      .split(" ")
      .map(parseFloat);

    const startY = e.clientY;
    const startH0 = computedRows[row] || 100;
    const startH1 = computedRows[row + 1] || 100;
    const totalH = startH0 + startH1;

    // Add a class to body to prevent text selection during resize
    document.body.style.cursor = "ns-resize";
    document.body.style.userSelect = "none";

    const onMove = (ev: MouseEvent) => {
      const delta = ev.clientY - startY;
      const newH0 = Math.max(40, Math.min(totalH - 40, startH0 + delta));
      const newH1 = totalH - newH0;

      // Update all row heights (preserve existing, fill missing with computed)
      const newHeights: number[] = [];
      for (let r = 0; r < maxRow; r++) {
        if (r === row) {
          newHeights.push(newH0);
        } else if (r === row + 1) {
          newHeights.push(newH1);
        } else {
          newHeights.push(heights[r] ?? computedRows[r] ?? 100);
        }
      }
      rowHeights.value = newHeights;
    };

    const onUp = () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup", onUp);
    };

    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup", onUp);
  }, [maxRow, heights]);

  // Calculate top positions for resize handles (percentage based on fr values)
  const totalFr = rowFrs.reduce((sum, fr) => sum + parseFloat(fr), 0);
  const resizeHandles: { row: number; topPercent: number }[] = [];
  let cumFr = 0;
  for (let r = 0; r < maxRow - 1; r++) {
    cumFr += parseFloat(rowFrs[r]);
    resizeHandles.push({ row: r, topPercent: (cumFr / totalFr) * 100 });
  }

  return (
    <div
      ref={gridRef}
      class="window-grid"
      style={{
        gridTemplateColumns: `repeat(${cols}, 1fr)`,
        gridTemplateRows: rowFrs.join(" "),
      }}
    >
      {visiblePanes.map((pane) => {
        // Locked panes auto-stretch to span all rows from their start
        const effectiveRowSpan = pane.locked
          ? Math.max(pane.gridRowSpan, maxRow - pane.gridRow)
          : pane.gridRowSpan;

        return (
          <div
            key={pane.id}
            class={
              `window-grid-cell` +
              (pane.minimized ? " window-grid-cell-minimized" : "") +
              (dragging && dragging !== pane.id ? " window-grid-cell-drop-target" : "")
            }
            style={{
              gridRow: `${pane.gridRow + 1} / span ${effectiveRowSpan}`,
              gridColumn: `${pane.gridCol + 1} / span ${pane.gridColSpan}`,
            }}
            onDragOver={handleDragOver}
            onDrop={(e) => handleDrop(e as DragEvent, pane.id)}
          >
            <PaneContainer paneId={pane.id} />
          </div>
        );
      })}

      {/* Row resize handles — overlaid at row boundaries */}
      {resizeHandles.map(({ row, topPercent }) => (
        <div
          key={`resize-row-${row}`}
          class="row-resize-handle"
          style={{ top: `calc(${topPercent}% - 4px)` }}
          onMouseDown={(e) => handleResizeStart(e as MouseEvent, row)}
        />
      ))}

      {/* Render popout placeholders (they render their own PopoutWindow) */}
      {poppedOutPanes.map((pane) => (
        <PaneContainer key={pane.id} paneId={pane.id} />
      ))}
    </div>
  );
}
