const path = require('node:path');
require('esbuild').buildSync({
  entryPoints: [path.join(__dirname, 'src/host.cjs')],
  outfile: path.resolve(__dirname, '../../../../../resource/hosts/raycast-host.js'),
  bundle: true,
  platform: 'node',
  target: 'node20',
  format: 'cjs',
  minify: true,
  define: { 'process.env.NODE_ENV': '"production"' },
  legalComments: 'eof'
});
