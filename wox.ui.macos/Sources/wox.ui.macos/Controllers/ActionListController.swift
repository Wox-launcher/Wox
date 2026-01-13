import Foundation

class ActionListController: BaseListController<WoxResultAction> {
    func updateActiveIndexByDirection(_ direction: Direction) {
        if items.isEmpty { return }
        
        var newIndex = activeIndex
        
        switch direction {
        case .down:
            newIndex += 1
            if newIndex >= items.count {
                newIndex = 0
            }
        case .up:
            newIndex -= 1
            if newIndex < 0 {
                newIndex = items.count - 1
            }
        case .left, .right:
            break
        }
        
        updateActiveIndex(newIndex)
    }
}
