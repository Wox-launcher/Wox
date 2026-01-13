import Foundation

class ResultListController: BaseListController<WoxQueryResult> {
    func updateActiveIndexByDirection(_ direction: Direction) {
        if items.isEmpty { return }
        
        var newIndex = activeIndex
        
        switch direction {
        case .down:
            newIndex += 1
            if newIndex >= items.count {
                newIndex = 0
            }
            
            var safetyCounter = 0
            while newIndex < items.count && items[newIndex].isGroup && safetyCounter < items.count {
                newIndex += 1
                safetyCounter += 1
                if newIndex >= items.count {
                    newIndex = 0
                }
            }
            
        case .up:
            newIndex -= 1
            if newIndex < 0 {
                newIndex = items.count - 1
            }
            
            var safetyCounter = 0
            while newIndex >= 0 && items[newIndex].isGroup && safetyCounter < items.count {
                newIndex -= 1
                safetyCounter += 1
                if newIndex < 0 {
                    newIndex = items.count - 1
                }
            }
            
        case .left:
            newIndex = findPrevNonGroupIndex(newIndex)
            
        case .right:
            newIndex = findNextNonGroupIndex(newIndex)
        }
        
        updateActiveIndex(newIndex)
    }
}
