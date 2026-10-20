const path = require('node:path');
const { unsupported } = require('./api.cjs');

const slot = (node, name) => node.children.find(child => child.type === 'slot' && child.props.name === name)?.children || [];
const descendants = nodes => nodes.flatMap(node => [node, ...descendants(node.children || [])]);

// Map React host instances to Wox's existing result/layout/form DTOs; never send a React tree to Go.
function createMapper(config, navigation) {
  let handlers = new Map();
  let controls = {};
  let lastSearch = '';
  let deliveredSearch = '';
  let lastRefinements = {};
  const image = value => {
    if (!value) return {};
    if (typeof value === 'object') value = value.source || value;
    if (typeof value === 'object') value = config.dark ? value.dark : value.light;
    if (typeof value !== 'string') return {};
    if (value.startsWith('wox-icon:')) return { ImageType: 'raycast', ImageData: value.slice(9) };
    if (/^https?:/.test(value)) return { ImageType: 'url', ImageData: value };
    if (/^data:/.test(value)) return { ImageType: 'base64', ImageData: value };
    return { ImageType: 'absolute', ImageData: path.isAbsolute(value) ? value : path.join(config.assetsPath, value) };
  };
  const shortcut = value => {
    if (!value) return '';
    const mods = { cmd: process.platform === 'darwin' ? 'Command' : 'Ctrl', ctrl: 'Ctrl', opt: 'Alt', shift: 'Shift' };
    return [...(value.modifiers || []).map(mod => mods[mod] || mod), value.key].join('+');
  };
  const preview = node => {
    if (!node) return {};
    let markdown = node.props.markdown || '';
    const metadata = descendants(slot(node, 'metadata'));
    for (const item of metadata) {
      if (item.type === 'Metadata.Label') markdown += `\n\n**${item.props.title || ''}** ${item.props.text || ''}`;
      if (item.type === 'Metadata.Link') markdown += `\n\n[${item.props.text || item.props.title || ''}](${item.props.target || ''})`;
      if (item.type === 'Metadata.Tag') markdown += ` · ${item.props.text || ''}`;
    }
    return { PreviewType: 'markdown', PreviewData: markdown };
  };
  const formFields = node => descendants(node.children).filter(item => item.type.startsWith('Form.') && !['Form.Description', 'Form.Separator'].includes(item.type));
  const form = node => formFields(node).map(field => {
    const p = field.props;
    if (p.onChange || p.onBlur || p.onFocus) unsupported('live form field callbacks');
    const types = { 'Form.TextField': 'textbox', 'Form.PasswordField': 'password', 'Form.TextArea': 'textbox', 'Form.Checkbox': 'checkbox', 'Form.Dropdown': 'select' };
    if (!types[field.type] || !p.id) unsupported(field.type);
    return { Type: types[field.type], Value: {
      Key: p.id, Label: p.title || p.label || p.id, Tooltip: p.info || '',
      DefaultValue: String(p.value ?? p.defaultValue ?? (field.type === 'Form.Checkbox' ? false : '')),
      MaxLines: field.type === 'Form.TextArea' ? 5 : 1,
      Options: descendants(field.children).filter(item => item.type === 'Dropdown.Item').map(item => ({ Label: item.props.title, Value: item.props.value, Icon: image(item.props.icon) }))
    } };
  });
  const actions = (nodes, formNode) => {
    const result = [];
    for (const node of descendants(nodes)) {
      if (!['Action', 'Action.SubmitForm'].includes(node.type)) continue;
      const p = node.props;
      const id = `action:${node.id}`;
      let fields;
      if (node.type === 'Action.SubmitForm') {
        if (!formNode) unsupported('SubmitForm outside Form');
        fields = form(formNode);
        handlers.set(id, values => {
          const typed = { ...values };
          for (const field of formFields(formNode)) if (field.type === 'Form.Checkbox') typed[field.props.id] = values[field.props.id] === 'true';
          return p.onSubmit?.(typed);
        });
      } else handlers.set(id, () => p.onAction?.());
      result.push({ Id: id, Type: fields ? 'form' : 'execute', Name: p.title || '', Icon: image(p.icon), Form: fields, Hotkey: shortcut(p.shortcut), PreventHideAfterAction: true });
    }
    if (navigation.depth() > 1) {
      handlers.set('navigation:back', navigation.pop);
      result.push({ Id: 'navigation:back', Type: 'execute', Name: 'i18n:plugin_raycast_back', PreventHideAfterAction: true });
    }
    return result;
  };
  return {
    map(container) {
      handlers = new Map();
      controls = {};
      const root = container.children.find(node => node.type !== '#text');
      if (!root) return { Results: [], Refinements: [], Layout: {} };
      if (!['List', 'Grid', 'Detail', 'Form'].includes(root.type)) unsupported(`root component ${root.type}`);
      if (root.props.onSelectionChange) unsupported('onSelectionChange');
      if (root.props.pagination) unsupported('pagination');
      const response = { Results: [], Refinements: [], Layout: {} };
      controls.search = root.props.onSearchTextChange;
      const dropdown = slot(root, 'searchBarAccessory').find(node => node.type === 'Dropdown');
      if (dropdown) {
        controls.dropdown = dropdown.props.onChange;
        const options = descendants(dropdown.children).filter(node => node.type === 'Dropdown.Item').map(node => ({ Value: node.props.value, Title: node.props.title, Icon: image(node.props.icon) }));
        response.Refinements.push({ Id: 'raycast.dropdown', Title: dropdown.props.tooltip || '', Type: 'singleSelect',
          DefaultValue: [dropdown.props.value ?? dropdown.props.defaultValue ?? options[0]?.Value ?? ''], Persist: !!dropdown.props.storeValue, Options: options });
      }
      if (root.type === 'Grid') response.Layout.GridLayout = { Columns: Math.max(1, Math.min(16, root.props.columns || 5)), ShowTitle: true, ItemMargin: 6, AspectRatio: root.props.aspectRatio || 1 };
      const rootActions = slot(root, 'actions');
      if (root.type === 'Detail' || root.type === 'Form') {
        response.Results.push({ Id: 'content', Title: root.props.navigationTitle || config.command.title, Preview: root.type === 'Detail' ? preview(root) : {}, Actions: actions(rootActions, root.type === 'Form' ? root : null) });
        if (root.type === 'Detail') response.Layout.ResultPreviewWidthRatio = 0;
      } else {
        let groupIndex = 0;
        const collect = (nodes, group = '', groupScore = 0) => {
          for (const node of nodes) {
            if (node.type.endsWith('.Section')) { collect(node.children, node.props.title || '', -groupIndex++); continue; }
            if (node.type !== `${root.type}.Item`) continue;
            const p = node.props;
            if ((root.props.filtering ?? !controls.search) && lastSearch) {
              const text = [p.title, p.subtitle, ...(p.keywords || [])].join(' ').toLowerCase();
              if (!lastSearch.toLowerCase().split(/\s+/).every(word => text.includes(word))) continue;
            }
            response.Results.push({ Id: String(p.id ?? node.id), Title: p.title || '', SubTitle: p.subtitle || '', Icon: image(p.content || p.icon),
              Group: group, GroupScore: groupScore, Score: -response.Results.length,
              Preview: preview(slot(node, 'detail')[0]),
              Tails: (p.accessories || []).map(item => ({ Type: 'text', Text: String(item.text ?? item.tag?.value ?? ''), Tooltip: item.tooltip || '' })),
              Actions: actions([...slot(node, 'actions'), ...rootActions]) });
          }
        };
        collect(root.children);
        if (root.props.isShowingDetail) response.Layout.ResultPreviewWidthRatio = 0.4;
        if (!response.Results.length) {
          const empty = root.children.find(node => node.type.endsWith('.EmptyView'));
          response.Results.push({ Id: 'empty', Title: empty?.props.title || (root.props.isLoading ? 'i18n:plugin_raycast_loading' : 'i18n:plugin_raycast_empty'), SubTitle: empty?.props.description || '', Actions: actions(rootActions) });
        }
      }
      return response;
    },
    async input(search, refinements) {
      lastSearch = search;
      if (controls.search && deliveredSearch !== search) {
        deliveredSearch = search;
        await controls.search(search);
      }
      const next = refinements['raycast.dropdown'];
      if (controls.dropdown && next !== undefined && next !== lastRefinements['raycast.dropdown']) {
        lastRefinements = { ...refinements };
        await controls.dropdown(next);
      }
    },
    async dispatch(id, values) {
      const handler = handlers.get(id);
      if (!handler) throw new Error('This Raycast action is no longer available');
      await handler(values);
    }
  };
}
module.exports = { createMapper };
