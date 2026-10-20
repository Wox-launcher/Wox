const React = require('react');
const { component } = require('./renderer.cjs');
const { createCache } = require('./cache.cjs');

const unsupported = name => { throw new Error(`Raycast ${name} is not supported by Wox`); };

// Expose only capabilities that have a Wox equivalent; unknown calls fail explicitly.
function createAPI(config, hostCall, navigation) {
  const api = {};
  for (const name of ['List', 'Grid', 'Detail', 'Form', 'ActionPanel', 'Action']) api[name] = component(name);
  for (const parent of ['List', 'Grid']) {
    api[parent].Item = component(`${parent}.Item`);
    api[parent].Section = component(`${parent}.Section`);
    api[parent].EmptyView = component(`${parent}.EmptyView`);
    api[parent].Dropdown = component('Dropdown');
    api[parent].Dropdown.Item = component('Dropdown.Item');
    api[parent].Dropdown.Section = component('Dropdown.Section');
  }
  api.List.Item.Detail = component('Detail');
  api.Detail.Metadata = component('Metadata');
  for (const type of ['Label', 'Link', 'Separator', 'TagList']) api.Detail.Metadata[type] = component(`Metadata.${type}`);
  api.Detail.Metadata.TagList.Item = component('Metadata.Tag');
  api.List.Item.Detail.Metadata = api.Detail.Metadata;
  for (const type of ['TextField', 'PasswordField', 'TextArea', 'Checkbox', 'Dropdown', 'Description', 'Separator']) api.Form[type] = component(`Form.${type}`);
  api.Form.Dropdown.Item = component('Dropdown.Item');
  api.Form.Dropdown.Section = component('Dropdown.Section');
  api.ActionPanel.Section = component('ActionPanel.Section');
  // Wox has a flat action panel; sections and submenus contribute their actions to that panel.
  api.ActionPanel.Submenu = component('ActionPanel.Section');
  const SubmitForm = component('Action.SubmitForm');
  api.Action.SubmitForm = props => React.createElement(SubmitForm, { ...props, title: props.title || 'i18n:plugin_raycast_submit' });
  api.Action.CopyToClipboard = props => React.createElement(api.Action, { ...props, title: props.title || 'i18n:plugin_raycast_copy', onAction: () => api.Clipboard.copy(props.content) });
  api.Action.OpenInBrowser = props => React.createElement(api.Action, { ...props, title: props.title || 'i18n:plugin_raycast_open_browser', onAction: () => api.open(props.url) });
  api.Action.Open = props => React.createElement(api.Action, { ...props, title: props.title || 'i18n:plugin_raycast_open', onAction: () => api.open(props.target) });
  api.Action.ShowInFinder = props => React.createElement(api.Action, { ...props, title: props.title || 'i18n:plugin_raycast_reveal', onAction: () => api.showInFinder(props.path) });
  api.Action.Push = props => React.createElement(api.Action, { ...props, onAction: () => navigation.push(props.target) });
  api.Action.Style = { Regular: 'regular', Destructive: 'destructive' };
  // Optional unsupported actions must not prevent otherwise usable result lists from rendering.
  for (const name of ['Paste', 'CreateQuicklink']) {
    api.Action[name] = props => React.createElement(api.Action, { ...props, title: props.title || 'i18n:plugin_raycast_unsupported_action', onAction: () => unsupported(`Action.${name}`) });
  }
  api.useNavigation = () => ({ push: navigation.push, pop: navigation.pop });
  api.environment = {
    commandName: config.command.name, extensionName: config.extensionName, ownerOrAuthorName: config.author,
    assetsPath: config.assetsPath, supportPath: config.supportPath,
    isDevelopment: false, appearance: config.dark ? 'dark' : 'light',
    launchType: 'userInitiated', commandMode: config.command.mode, raycastVersion: '1.0.0'
  };
  api.getPreferenceValues = () => ({ ...config.preferences });
  api.Clipboard = {
    copy: async value => {
      const text = typeof value === 'string' || typeof value === 'number' ? String(value) : value?.text;
      if (text == null) return unsupported('Clipboard.copy with non-text content');
      return hostCall('copy', { text });
    },
    readText: () => hostCall('readClipboard', {}),
    read: async () => ({ text: await hostCall('readClipboard', {}) }),
    paste: () => unsupported('Clipboard.paste')
  };
  api.open = (target, application) => application ? unsupported('open with a specific application') : hostCall('open', { target: String(target) });
  api.showInFinder = target => hostCall('reveal', { target });
  api.closeMainWindow = () => hostCall('hide', {});
  api.showHUD = title => hostCall('notify', { title });
  api.showToast = async (options, title, message) => {
    if (typeof options === 'string') options = { style: options, title, message };
    const toast = { ...options, hide: async () => {} };
    await hostCall('notify', { title: [toast.title, toast.message].filter(Boolean).join(': ') });
    return toast;
  };
  api.Toast = { Style: { Animated: 'animated', Success: 'success', Failure: 'failure' } };
  api.popToRoot = async () => navigation.root();
  api.clearSearchBar = () => hostCall('clearSearch', {});
  api.getSelectedText = () => hostCall('selectedText', {});
  api.LocalStorage = {
    getItem: async key => (await hostCall('storageGet', { key })) ?? undefined,
    setItem: (key, value) => hostCall('storageSet', { key, value }),
    removeItem: key => hostCall('storageRemove', { key }),
    allItems: () => hostCall('storageAll', {}),
    clear: () => hostCall('storageClear', {})
  };
  // Cache is deliberately separate from settings and never participates in Cloud Sync.
  api.Cache = createCache(config.supportPath);
  api.Icon = new Proxy({}, { get: (_, key) => `wox-icon:${String(key)}` });
  api.Color = { PrimaryText: 'primary', SecondaryText: 'secondary', Red: 'red', Green: 'green', Blue: 'blue', Yellow: 'yellow', Orange: 'orange', Purple: 'purple', Magenta: 'magenta' };
  api.Image = { Mask: { Circle: 'circle', RoundedRectangle: 'roundedRectangle' } };
  api.Grid.ItemSize = { Small: 'small', Medium: 'medium', Large: 'large' };
  api.Grid.Fit = { Fill: 'fill', Contain: 'contain' };
  api.LaunchType = { UserInitiated: 'userInitiated', Background: 'background' };
  api.Keyboard = { Shortcut: { Common: {
    Copy: { modifiers: ['cmd'], key: 'c' }, Open: { modifiers: ['cmd'], key: 'o' },
    Refresh: { modifiers: ['cmd'], key: 'r' }, Remove: { modifiers: ['ctrl'], key: 'x' }
  } } };
  // Report only implemented capabilities; bundled utilities use this to hide AI-only controls.
  const accessible = new Set([api.Clipboard, api.Clipboard.copy, api.Clipboard.readText, api.LocalStorage, api.Cache, api.open, api.getSelectedText]);
  api.environment.canAccess = capability => accessible.has(capability);
  return new Proxy(api, { get: (target, key) => {
    if (key in target || typeof key === 'symbol' || ['then', '__esModule', 'default'].includes(key)) return target[key];
    // Preserve import-time property access, but fail on calling or rendering unsupported APIs.
    const fail = function () { return unsupported(String(key)); };
    return new Proxy(fail, { get: () => fail });
  } });
}

module.exports = { createAPI, unsupported };
