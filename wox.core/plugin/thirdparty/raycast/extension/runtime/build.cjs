const fs = require('node:fs');
const path = require('node:path');
const core = path.resolve(__dirname, '../../../../..');
const { metafile } = require('esbuild').buildSync({
  entryPoints: [path.join(__dirname, 'src/host.cjs')],
  outfile: path.join(core, 'resource/hosts/raycast-host.js'),
  bundle: true,
  platform: 'node',
  target: 'node20',
  format: 'cjs',
  minify: true,
  metafile: true,
  define: { 'process.env.NODE_ENV': '"production"' },
  legalComments: 'eof'
});

// Read the manifests of packages that actually contributed code to the bundle.
const entries = new Map();
for (const output of Object.values(metafile.outputs)) {
  for (const [input, { bytesInOutput }] of Object.entries(output.inputs)) {
    if (!bytesInOutput || !input.split(/[\\/]/).includes('node_modules')) continue;
    let dir = path.dirname(path.resolve(input));
    while (!fs.existsSync(path.join(dir, 'package.json'))) {
      const parent = path.dirname(dir);
      if (dir === parent) throw new Error(`Cannot find the package for ${input}`);
      dir = parent;
    }
    const { name, version } = JSON.parse(fs.readFileSync(path.join(dir, 'package.json'), 'utf8'));
    const id = `${name}@${version}`;
    if (entries.has(id)) continue;
    const files = fs.readdirSync(dir).filter(name => /^(licen[cs]e|copying|notice|copyright)([._-]|$)/i.test(name)).sort();
    if (!files.length) throw new Error(`Cannot find the license for ${id}`);
    const text = files.map(name => `${name}\n\n${fs.readFileSync(path.join(dir, name), 'utf8').trim()}`).join('\n\n');
    entries.set(id, { name, version, text });
  }
}
const licensesDir = path.join(core, '.build/licenses');
fs.mkdirSync(licensesDir, { recursive: true });
fs.writeFileSync(path.join(licensesDir, 'raycast-host.json'), JSON.stringify([...entries.values()]));
