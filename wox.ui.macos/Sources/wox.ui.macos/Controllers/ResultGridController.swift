import Foundation
import CoreGraphics

class ResultGridController: BaseListController<WoxQueryResult> {
    @Published var gridLayoutParams: GridLayoutParams = .empty()
    @Published var rowHeight: CGFloat = 0
    
    func updateGridParams(_ params: GridLayoutParams) {
        self.gridLayoutParams = params
    }
    
    func updateRowHeight(_ height: CGFloat) {
        if abs(self.rowHeight - height) < 0.5 {
            return
        }
        DispatchQueue.main.async {
            self.rowHeight = height
        }
    }
    
    func updateActiveIndexByDirection(_ direction: Direction) {
        if items.isEmpty { return }
        
        var newIndex = activeIndex
        
        switch direction {
        case .down:
            newIndex = findNextRowIndex(currentIndex: newIndex)
        case .up:
            newIndex = findPrevRowIndex(currentIndex: newIndex)
        case .left:
            newIndex = findPrevNonGroupIndex(newIndex)
        case .right:
            newIndex = findNextNonGroupIndex(newIndex)
        }
        
        updateActiveIndex(newIndex)
    }
    
    private func findNextRowIndex(currentIndex: Int) -> Int {
        let rows = buildGridRows()
        if rows.isEmpty { return currentIndex }
        
        var currentRow = -1
        var currentCol = -1
        
        for (r, rowItems) in rows.enumerated() {
            if let col = rowItems.firstIndex(of: currentIndex) {
                currentRow = r
                currentCol = col
                break
            }
        }
        
        if currentRow == -1 { return currentIndex }
        
        var nextRow = currentRow + 1
        if nextRow >= rows.count {
            nextRow = 0
        }
        
        let nextRowItems = rows[nextRow]
        if nextRowItems.isEmpty { return currentIndex }
        
        if currentCol < nextRowItems.count {
            return nextRowItems[currentCol]
        } else {
            return nextRowItems.last ?? currentIndex
        }
    }
    
    private func findPrevRowIndex(currentIndex: Int) -> Int {
        let rows = buildGridRows()
        if rows.isEmpty { return currentIndex }
        
        var currentRow = -1
        var currentCol = -1
        
        for (r, rowItems) in rows.enumerated() {
            if let col = rowItems.firstIndex(of: currentIndex) {
                currentRow = r
                currentCol = col
                break
            }
        }
        
        if currentRow == -1 { return currentIndex }
        
        var prevRow = currentRow - 1
        if prevRow < 0 {
            prevRow = rows.count - 1
        }
        
        let prevRowItems = rows[prevRow]
        if prevRowItems.isEmpty { return currentIndex }
        
        if currentCol < prevRowItems.count {
            return prevRowItems[currentCol]
        } else {
            return prevRowItems.last ?? currentIndex
        }
    }
    
    private func buildGridRows() -> [[Int]] {
        var rows: [[Int]] = []
        let columns = max(1, gridLayoutParams.columns)
        var i = 0
        
        while i < items.count {
            if items[i].isGroup {
                i += 1
            } else {
                var rowIndices: [Int] = []
                let currentGroup = items[i].data.group
                while i < items.count && !items[i].isGroup && items[i].data.group == currentGroup && rowIndices.count < columns {
                    rowIndices.append(i)
                    i += 1
                }
                if !rowIndices.isEmpty {
                    rows.append(rowIndices)
                }
            }
        }
        return rows
    }
    
    func calculateGridHeight() -> CGFloat {
        if items.isEmpty || gridLayoutParams.columns <= 0 || rowHeight <= 0 {
            return 0
        }
        
        let groupHeaderHeight: CGFloat = 32.0
        var totalHeight: CGFloat = 0
        var i = 0
        let columns = gridLayoutParams.columns
        
        while i < items.count {
            if items[i].isGroup {
                totalHeight += groupHeaderHeight
                i += 1
            } else {
                let currentGroup = items[i].data.group
                var itemsInRow = 0
                while i < items.count && !items[i].isGroup && items[i].data.group == currentGroup && itemsInRow < columns {
                    itemsInRow += 1
                    i += 1
                }
                if itemsInRow > 0 {
                    totalHeight += rowHeight
                }
            }
        }
        return totalHeight
    }
}
