import SwiftUI

/// Grid view for displaying results in a grid layout
struct GridView: View {
    @ObservedObject var viewModel: WoxViewModel
    @ObservedObject var controller: ResultGridController
    let maxHeight: CGFloat
    
    private var gridLayoutParams: GridLayoutParams {
        controller.gridLayoutParams
    }
    
    private var columns: Int {
        gridLayoutParams.columns
    }
    
    private var showTitle: Bool {
        gridLayoutParams.showTitle
    }
    
    private var itemPadding: CGFloat {
        gridLayoutParams.itemPadding
    }
    
    private var itemMargin: CGFloat {
        gridLayoutParams.itemMargin
    }
    
    var body: some View {
        GeometryReader { geometry in
            let availableWidth = geometry.size.width
            let cellWidth = columns > 0 ? (availableWidth / CGFloat(columns)).rounded(.down) : 48
            let iconSize = cellWidth - (itemPadding + itemMargin) * 2
            let titleHeight: CGFloat = showTitle ? 18 : 0
            let cellHeight = cellWidth + titleHeight
            
            let _ = controller.updateRowHeight(cellHeight)
            
            ScrollViewReader { proxy in
                ScrollView(.vertical, showsIndicators: false) {
                    VStack(spacing: 0) {
                        let items = controller.items
                        
                        ForEach(0..<items.count, id: \.self) { index in
                            let item = items[index]
                            
                            if item.isGroup {
                                // Group header
                                HStack {
                                    Text(item.title)
                                        .font(.system(size: 12, weight: .medium))
                                        .foregroundColor(Color(hex: viewModel.theme.resultItemSubTitleColor))
                                    Spacer()
                                }
                                .padding(.leading, 8)
                                .padding(.top, 12)
                                .padding(.bottom, 4)
                            }
                        }
                        
                        // Build grid rows
                        buildGridRows(
                            items: items.filter { !$0.isGroup },
                            cellWidth: cellWidth,
                            cellHeight: cellHeight,
                            iconSize: iconSize
                        )
                    }
                }
                .onChange(of: controller.activeIndex) { newIndex in
                    if newIndex < controller.items.count {
                        proxy.scrollTo(controller.items[newIndex].id, anchor: .center)
                    }
                }
            }
        }
        .frame(maxHeight: maxHeight)
    }
    
    @ViewBuilder
    private func buildGridRows(items: [WoxListItem<WoxQueryResult>], cellWidth: CGFloat, cellHeight: CGFloat, iconSize: CGFloat) -> some View {
        let rows = stride(from: 0, to: items.count, by: columns).map { startIndex in
            Array(items[startIndex..<min(startIndex + columns, items.count)])
        }
        
        ForEach(0..<rows.count, id: \.self) { rowIndex in
            HStack(spacing: 0) {
                ForEach(0..<columns, id: \.self) { colIndex in
                    if colIndex < rows[rowIndex].count {
                        let item = rows[rowIndex][colIndex]
                        let globalIndex = rowIndex * columns + colIndex
                        
                        GridItemView(
                            result: item.data,
                            isSelected: viewModel.selectedResultId == item.id,
                            isHovered: controller.hoveredIndex == globalIndex,
                            iconSize: iconSize,
                            showTitle: showTitle,
                            itemPadding: itemPadding,
                            itemMargin: itemMargin,
                            theme: viewModel.theme,
                            quickSelectNumber: quickSelectNumber(for: globalIndex)
                        )
                        .frame(width: cellWidth, height: cellHeight)
                        .onTapGesture {
                            viewModel.selectedResultId = item.id
                            viewModel.executeAction(result: item.data)
                        }
                        .onHover { hovering in
                            if hovering {
                                controller.hoveredIndex = globalIndex
                            } else if controller.hoveredIndex == globalIndex {
                                controller.hoveredIndex = -1
                            }
                        }
                    } else {
                        Spacer()
                            .frame(width: cellWidth, height: cellHeight)
                    }
                }
            }
        }
    }
    
    private func quickSelectNumber(for index: Int) -> Int? {
        guard viewModel.isQuickSelectMode, index < 9 else { return nil }
        return index + 1
    }
}

struct GridItemView: View {
    let result: WoxQueryResult
    let isSelected: Bool
    let isHovered: Bool
    let iconSize: CGFloat
    let showTitle: Bool
    let itemPadding: CGFloat
    let itemMargin: CGFloat
    let theme: WoxTheme
    let quickSelectNumber: Int?
    
    var body: some View {
        VStack(spacing: 0) {
            ZStack(alignment: .topLeading) {
                WoxIconView(icon: result.icon, size: iconSize)
                    .padding(itemPadding)
                    .background(
                        RoundedRectangle(cornerRadius: 8)
                            .fill(backgroundColor)
                    )
                
                // Quick Select number badge
                if let number = quickSelectNumber {
                    Text("\(number)")
                        .font(.system(size: 10, weight: .bold))
                        .foregroundColor(.white)
                        .frame(width: 16, height: 16)
                        .background(Circle().fill(Color(hex: theme.resultItemActiveBackgroundColor)))
                        .offset(x: -2, y: -2)
                }
            }
            .padding(itemMargin)
            
            if showTitle {
                Text(result.title)
                    .font(.system(size: 12))
                    .foregroundColor(Color(hex: theme.resultItemTitleColor))
                    .lineLimit(1)
                    .padding(.horizontal, itemMargin)
            }
        }
    }
    
    private var backgroundColor: Color {
        if isSelected {
            return Color(hex: theme.resultItemActiveBackgroundColor)
        } else if isHovered {
            return Color(hex: theme.resultItemActiveBackgroundColor).opacity(0.3)
        }
        return .clear
    }
}
