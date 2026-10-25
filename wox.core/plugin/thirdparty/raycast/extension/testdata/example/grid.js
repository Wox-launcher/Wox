const React = require('react');
const { Grid, Action, ActionPanel, Icon } = require('@raycast/api');
const h = React.createElement;

exports.default = () => h(Grid, { columns: 3 }, ...['Clipboard', 'Folder', 'Star'].map(name =>
  h(Grid.Item, {
    key: name, id: name, title: name, content: Icon[name],
    actions: h(ActionPanel, null, h(Action.CopyToClipboard, { content: name }))
  })));
