const React = require('react');
const { List, Action, ActionPanel } = require('@raycast/api');
const h = React.createElement;

// Keep the example prebuilt so local import needs no package installation or compiler.
exports.default = function Command() {
  const [category, setCategory] = React.useState('all');
  const [loaded, setLoaded] = React.useState(false);
  React.useEffect(() => {
    const timer = setTimeout(() => setLoaded(true), 600);
    return () => clearTimeout(timer);
  }, []);
  return h(List, {
    isShowingDetail: true,
    searchBarAccessory: h(List.Dropdown, { value: category, onChange: setCategory },
      h(List.Dropdown.Item, { value: 'all', title: 'All' }),
      h(List.Dropdown.Item, { value: 'fruit', title: 'Fruit' }))
  }, ...['Apple', 'Banana', 'Carrot'].filter(title => category === 'all' || title !== 'Carrot').map(title =>
    h(List.Item, {
      key: title, id: title, title, subtitle: loaded ? 'Loaded asynchronously' : 'Loading…',
      detail: h(List.Item.Detail, { markdown: `# ${title}\n\nThis is Wox's existing Markdown preview.` }),
      actions: h(ActionPanel, null, h(Action.CopyToClipboard, { content: title }))
    })));
};
