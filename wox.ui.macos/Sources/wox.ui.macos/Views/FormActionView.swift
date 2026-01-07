import SwiftUI
import AppKit

// MARK: - Form Action Panel View

struct FormActionPanelView: View {
    let action: WoxResultAction
    @Binding var values: [String: String]
    let theme: WoxTheme
    let onSave: ([String: String]) -> Void
    let onCancel: () -> Void
    
    @State private var localValues: [String: String] = [:]
    @FocusState private var isTextFieldFocused: Bool
    
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            // Header
            HStack {
                Text(action.name)
                    .font(.system(size: 14, weight: .medium))
                    .foregroundColor(Color(hex: theme.actionContainerHeaderFontColor))
                Spacer()
            }
            
            Divider()
                .background(Color(hex: theme.previewSplitLineColor))
            
            // Form Fields
            ScrollView(.vertical, showsIndicators: false) {
                VStack(alignment: .leading, spacing: 10) {
                    if let form = action.form {
                        ForEach(form.indices, id: \.self) { index in
                            buildField(form[index])
                        }
                    }
                }
            }
            .frame(maxHeight: 300)
            
            Divider()
                .background(Color(hex: theme.previewSplitLineColor))
            
            // Buttons
            HStack {
                Spacer()
                
                Button(action: onCancel) {
                    Text("Cancel (Esc)")
                        .font(.system(size: 12))
                        .padding(.horizontal, 16)
                        .padding(.vertical, 8)
                }
                .buttonStyle(.plain)
                .background(Color.gray.opacity(0.2))
                .cornerRadius(6)
                
                Button(action: { onSave(localValues) }) {
                    Text("Save (⌘+Enter)")
                        .font(.system(size: 12))
                        .padding(.horizontal, 16)
                        .padding(.vertical, 8)
                        .foregroundColor(.white)
                }
                .buttonStyle(.plain)
                .background(Color.blue)
                .cornerRadius(6)
                .keyboardShortcut(.return, modifiers: .command)
            }
        }
        .padding(16)
        .frame(width: 400)
        .background(Color(hex: theme.actionContainerBackgroundColor))
        .background(VisualEffectView(material: .hudWindow, blendingMode: .behindWindow))
        .cornerRadius(12)
        .shadow(color: Color.black.opacity(0.3), radius: 10, x: 0, y: 5)
        .overlay(
            RoundedRectangle(cornerRadius: 12)
                .stroke(Color.white.opacity(0.15), lineWidth: 0.5)
        )
        .onAppear {
            // Initialize local values from binding
            localValues = values
            // Initialize default values from form
            if let form = action.form {
                for item in form {
                    let key = getFormItemKey(item)
                    let defaultValue = getFormItemDefaultValue(item)
                    if !key.isEmpty && localValues[key] == nil {
                        localValues[key] = defaultValue
                    }
                }
            }
        }
    }
    
    @ViewBuilder
    private func buildField(_ item: PluginSettingDefinitionItem) -> some View {
        switch item.type {
        case "textbox":
            buildTextbox(item)
        case "checkbox":
            buildCheckbox(item)
        case "select":
            buildSelect(item)
        case "head":
            buildHead(item)
        case "label":
            buildLabel(item)
        case "newline":
            Spacer().frame(height: 8)
        default:
            Text("Unsupported field type: \(item.type)")
                .font(.system(size: 12))
                .foregroundColor(.gray)
        }
    }
    
    private func buildTextbox(_ item: PluginSettingDefinitionItem) -> some View {
        let key = getFormItemKey(item)
        let label = getFormItemLabel(item)
        let tooltip = getFormItemTooltip(item)
        
        return VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .center, spacing: 10) {
                Text(label)
                    .font(.system(size: 13, weight: .medium))
                    .foregroundColor(Color(hex: theme.actionItemFontColor).opacity(0.9))
                    .frame(width: 80, alignment: .leading)
                
                TextField("", text: Binding(
                    get: { localValues[key] ?? "" },
                    set: { localValues[key] = $0 }
                ))
                .textFieldStyle(.plain)
                .font(.system(size: 13))
                .padding(.horizontal, 8)
                .padding(.vertical, 6)
                .background(Color.white.opacity(0.1))
                .cornerRadius(4)
                .focused($isTextFieldFocused)
            }
            
            if !tooltip.isEmpty {
                Text(tooltip)
                    .font(.system(size: 11))
                    .foregroundColor(Color(hex: theme.actionItemFontColor).opacity(0.6))
                    .padding(.leading, 90)
            }
        }
    }
    
    private func buildCheckbox(_ item: PluginSettingDefinitionItem) -> some View {
        let key = getFormItemKey(item)
        let label = getFormItemLabel(item)
        let tooltip = getFormItemTooltip(item)
        let isChecked = localValues[key] == "true"
        
        return VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .center, spacing: 10) {
                Text(label)
                    .font(.system(size: 13, weight: .medium))
                    .foregroundColor(Color(hex: theme.actionItemFontColor).opacity(0.9))
                    .frame(width: 80, alignment: .leading)
                
                Toggle("", isOn: Binding(
                    get: { isChecked },
                    set: { localValues[key] = $0 ? "true" : "false" }
                ))
                .toggleStyle(.checkbox)
            }
            
            if !tooltip.isEmpty {
                Text(tooltip)
                    .font(.system(size: 11))
                    .foregroundColor(Color(hex: theme.actionItemFontColor).opacity(0.6))
                    .padding(.leading, 90)
            }
        }
    }
    
    private func buildSelect(_ item: PluginSettingDefinitionItem) -> some View {
        let key = getFormItemKey(item)
        let label = getFormItemLabel(item)
        let tooltip = getFormItemTooltip(item)
        let options = getFormItemOptions(item)
        
        return VStack(alignment: .leading, spacing: 4) {
            HStack(alignment: .center, spacing: 10) {
                Text(label)
                    .font(.system(size: 13, weight: .medium))
                    .foregroundColor(Color(hex: theme.actionItemFontColor).opacity(0.9))
                    .frame(width: 80, alignment: .leading)
                
                Picker("", selection: Binding(
                    get: { localValues[key] ?? "" },
                    set: { localValues[key] = $0 }
                )) {
                    ForEach(options, id: \.value) { option in
                        Text(option.label).tag(option.value)
                    }
                }
                .pickerStyle(.menu)
            }
            
            if !tooltip.isEmpty {
                Text(tooltip)
                    .font(.system(size: 11))
                    .foregroundColor(Color(hex: theme.actionItemFontColor).opacity(0.6))
                    .padding(.leading, 90)
            }
        }
    }
    
    private func buildHead(_ item: PluginSettingDefinitionItem) -> some View {
        let content = getFormItemContent(item)
        
        return Text(content)
            .font(.system(size: 14, weight: .semibold))
            .foregroundColor(Color(hex: theme.actionItemFontColor))
            .padding(.top, 8)
    }
    
    private func buildLabel(_ item: PluginSettingDefinitionItem) -> some View {
        let content = getFormItemContent(item)
        
        return Text(content)
            .font(.system(size: 12))
            .foregroundColor(Color(hex: theme.actionItemFontColor).opacity(0.7))
            .padding(.leading, 90)
    }
    
    // MARK: - Helper functions to extract form item properties
    
    private func getFormItemKey(_ item: PluginSettingDefinitionItem) -> String {
        // This is a simplified extraction - in real implementation would parse the JSON value
        return item.value
    }
    
    private func getFormItemLabel(_ item: PluginSettingDefinitionItem) -> String {
        return "Label"
    }
    
    private func getFormItemTooltip(_ item: PluginSettingDefinitionItem) -> String {
        return ""
    }
    
    private func getFormItemContent(_ item: PluginSettingDefinitionItem) -> String {
        return item.value
    }
    
    private func getFormItemDefaultValue(_ item: PluginSettingDefinitionItem) -> String {
        return ""
    }
    
    private func getFormItemOptions(_ item: PluginSettingDefinitionItem) -> [(value: String, label: String)] {
        return []
    }
}
