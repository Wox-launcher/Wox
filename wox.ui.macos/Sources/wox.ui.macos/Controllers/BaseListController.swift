import Foundation
import Combine
import SwiftUI

class BaseListController<T>: ObservableObject {
    @Published var originalItems: [WoxListItem<T>] = []
    @Published var items: [WoxListItem<T>] = []
    @Published var activeIndex: Int = 0
    @Published var hoveredIndex: Int = -1
    
    @Published var filterText: String = ""
    
    var onItemExecuted: ((WoxListItem<T>) -> Void)?
    var onItemActive: ((WoxListItem<T>) -> Void)?
    var onItemsEmpty: (() -> Void)?
    
    var isMouseMoved = false
    
    func updateItems(_ newItems: [WoxListItem<T>], silent: Bool = false) {
        self.originalItems = newItems
        filterItems(filterText, silent: silent)
    }
    
    func updateActiveIndex(_ index: Int, silent: Bool = false) {
        guard index >= 0 && index < items.count else { return }
        activeIndex = index
        
        if !silent {
            onItemActive?(items[index])
        }
    }
    
    func clearHoveredResult() {
        hoveredIndex = -1
    }
    
    func filterItems(_ text: String, silent: Bool = false) {
        self.filterText = text
        if text.isEmpty {
            items = originalItems
        } else {
            let matchedItems = originalItems.filter { !$0.isGroup && isFuzzyMatch(text: $0.title, pattern: text) }
            items = findItemsToInclude(matchedItems: matchedItems)
        }
        
        if items.isEmpty {
            onItemsEmpty?()
        } else {
            updateActiveIndex(0, silent: silent)
        }
    }
    
    private func isFuzzyMatch(text: String, pattern: String) -> Bool {
        if pattern.isEmpty { return true }
        let textLower = text.lowercased()
        let patternLower = pattern.lowercased()
        
        var textIdx = textLower.startIndex
        var patternIdx = patternLower.startIndex
        
        while textIdx < textLower.endIndex && patternIdx < patternLower.endIndex {
            if textLower[textIdx] == patternLower[patternIdx] {
                patternIdx = textLower.index(after: patternIdx)
            }
            textIdx = textLower.index(after: textIdx)
        }
        
        return patternIdx == patternLower.endIndex
    }
    
    private func findItemsToInclude(matchedItems: [WoxListItem<T>]) -> [WoxListItem<T>] {
        let matchedItemIds = Set(matchedItems.map { $0.id })
        var groupsWithMatchingChildren = Set<String>()
        
        var currentGroupId: String?
        for item in originalItems {
            if item.isGroup {
                currentGroupId = item.id
            } else if matchedItemIds.contains(item.id), let groupId = currentGroupId {
                groupsWithMatchingChildren.insert(groupId)
            }
        }
        
        return originalItems.filter { item in
            if item.isGroup {
                return groupsWithMatchingChildren.contains(item.id)
            } else {
                return matchedItemIds.contains(item.id)
            }
        }
    }
    
    func findPrevNonGroupIndex(_ currentIndex: Int) -> Int {
        if items.isEmpty { return 0 }
        var newIndex = currentIndex - 1
        if newIndex < 0 {
            newIndex = items.count - 1
        }
        
        var safetyCounter = 0
        while items[newIndex].isGroup && safetyCounter < items.count {
            newIndex -= 1
            safetyCounter += 1
            if newIndex < 0 {
                newIndex = items.count - 1
            }
        }
        return newIndex
    }
    
    func findNextNonGroupIndex(_ currentIndex: Int) -> Int {
        if items.isEmpty { return 0 }
        var newIndex = currentIndex + 1
        if newIndex >= items.count {
            newIndex = 0
        }
        
        var safetyCounter = 0
        while items[newIndex].isGroup && safetyCounter < items.count {
            newIndex += 1
            safetyCounter += 1
            if newIndex >= items.count {
                newIndex = 0
            }
        }
        return newIndex
    }
}
