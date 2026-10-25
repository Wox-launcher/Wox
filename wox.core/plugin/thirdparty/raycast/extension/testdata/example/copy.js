const { Clipboard, showHUD } = require('@raycast/api');

exports.default = async () => {
  await Clipboard.copy('Hello from a local Raycast command');
  await showHUD('Copied');
};
