const React = require('react');
const { Form, Action, ActionPanel, Clipboard } = require('@raycast/api');
const h = React.createElement;

exports.default = () => h(Form, {
  actions: h(ActionPanel, null, h(Action.SubmitForm, {
    title: 'Edit and Copy', onSubmit: values => Clipboard.copy(JSON.stringify(values))
  }))
}, h(Form.TextField, { id: 'name', title: 'Name', defaultValue: 'Wox' }),
h(Form.Checkbox, { id: 'enabled', title: 'Enabled', defaultValue: true }));
