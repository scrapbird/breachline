// Generate the third-party license manifest bundled into the app + shown in the
// About dialog (Help > About > Third-party licenses).
//
// Covers the two ecosystems that ship in the distributed binary:
//   - Go   : only modules actually LINKED into the app binary (`go list -deps .`),
//            not the wider `go list -m all` graph (that pulls in Wails-CLI/test
//            tooling that never ships). License text is read from the module cache.
//   - npm  : production dependencies only (`npm ls --omit=dev`), text read from
//            each package's LICENSE file (falling back to the package.json field).
//
// Output: src/assets/third-party-licenses.json  (imported by AboutDialog).
//
// Runs automatically as the frontend `prebuild` step, so `wails build`, the CI
// release jobs, and `build.sh` all regenerate it. Node (not Python) so the same
// command works on the Windows/macOS/Linux CI runners without a python3-vs-python
// split. Run manually with `node scripts/gen-third-party-licenses.mjs`; pass
// `--check` to fail (non-zero exit) if the committed file is stale.
//
// stdlib only; requires `go` on PATH (for the linked-module list) and an
// installed node_modules tree.

import { execFileSync } from 'node:child_process';
import { readdirSync, readFileSync, existsSync, writeFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join, resolve } from 'node:path';

const SCRIPT_DIR = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(SCRIPT_DIR, '..');
const APP_DIR = resolve(FRONTEND_DIR, '..');
const OUTPUT = join(FRONTEND_DIR, 'src', 'assets', 'third-party-licenses.json');
const CHECK = process.argv.includes('--check');

const LICENSE_FILENAMES = [
    'LICENSE', 'LICENSE.txt', 'LICENSE.md', 'LICENCE', 'LICENCE.txt',
    'COPYING', 'COPYING.txt', 'LICENSE-MIT', 'License.txt',
];

// Classify license text into an SPDX-ish label. Order matters: the most
// restrictive families are tested first so a stray "GPL" mention in an
// otherwise-permissive file cannot be misread as permissive.
function classify(text) {
    const t = text.toLowerCase();
    if (t.includes('affero')) return 'AGPL';
    if (t.includes('gnu general public license') && !t.includes('lesser') && !t.includes('affero')) return 'GPL';
    if (t.includes('lesser general public')) return 'LGPL';
    if (t.includes('mozilla public license')) return 'MPL-2.0';
    if (t.includes('eclipse public license')) return 'EPL';
    if (t.includes('common development and distribution')) return 'CDDL';
    if (t.includes('apache license')) return 'Apache-2.0';
    if (t.includes('permission is hereby granted, free of charge')) return 'MIT';
    if (t.includes('redistribution and use in source and binary')) {
        return t.includes('neither the name') ? 'BSD-3-Clause' : 'BSD-2-Clause';
    }
    if (t.includes('permission to use, copy, modify')) return 'ISC';
    if (t.includes('unlicense') || t.includes('public domain')) return 'Unlicense';
    if (t.includes('creative commons') && t.includes('attribution')) return 'CC-BY-4.0';
    return 'UNKNOWN';
}

// Find and read the primary license file(s) in a directory. Apache NOTICE files
// are appended when present (Apache-2.0 section 4d requires propagating them).
function readLicenseDir(dir) {
    if (!dir || !existsSync(dir)) return null;
    let names;
    try {
        names = readdirSync(dir);
    } catch {
        return null;
    }
    const licenseFiles = [];
    for (const preferred of LICENSE_FILENAMES) {
        const hit = names.find((n) => n.toLowerCase() === preferred.toLowerCase());
        if (hit) { licenseFiles.push(hit); break; }
    }
    if (licenseFiles.length === 0) {
        const fuzzy = names.find((n) => /^(licen[sc]e|copying)/i.test(n));
        if (fuzzy) licenseFiles.push(fuzzy);
    }
    if (licenseFiles.length === 0) return null;

    let text = readFileSync(join(dir, licenseFiles[0]), 'utf8').trim();
    const notice = names.find((n) => n.toLowerCase() === 'notice' || /^notice(\.|$)/i.test(n));
    if (notice) {
        try {
            const noticeText = readFileSync(join(dir, notice), 'utf8').trim();
            if (noticeText) text += `\n\n----- NOTICE -----\n\n${noticeText}`;
        } catch { /* ignore unreadable NOTICE */ }
    }
    return text;
}

// go module cache escapes uppercase letters as !<lowercase> (e.g. Microsoft -> !microsoft).
function escapeModulePath(p) {
    return p.replace(/[A-Z]/g, (c) => '!' + c.toLowerCase());
}

function goEntries() {
    const goModCache = execFileSync('go', ['env', 'GOMODCACHE'], { cwd: APP_DIR, encoding: 'utf8' }).trim();
    // Only modules linked into the binary built from `.` (main package).
    const raw = execFileSync(
        'go',
        ['list', '-deps', '-f', '{{with .Module}}{{.Path}}@{{.Version}}{{end}}', '.'],
        { cwd: APP_DIR, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 },
    );
    const mods = [...new Set(raw.split('\n').map((l) => l.trim()).filter(Boolean))];
    const entries = [];
    for (const mod of mods) {
        const at = mod.lastIndexOf('@');
        const path = mod.slice(0, at);
        const version = mod.slice(at + 1);
        // Skip our own modules (local replaces, no third-party license to attribute).
        if (path === 'breachline' || path.startsWith('github.com/scrapbird/breachline')) continue;
        const dir = join(goModCache, escapeModulePath(path) + '@' + version);
        const text = readLicenseDir(dir);
        if (!text) {
            console.warn(`[licenses] WARN: no license file found for Go module ${mod} (looked in ${dir})`);
            continue;
        }
        entries.push({ name: path, version, source: 'go', license: classify(text), text });
    }
    return entries;
}

function npmEntries() {
    // Production dependency package directories, absolute paths, one per line.
    const raw = execFileSync(
        'npm',
        ['ls', '--omit=dev', '--all', '--parseable', '--long=false'],
        { cwd: FRONTEND_DIR, encoding: 'utf8', maxBuffer: 32 * 1024 * 1024 },
    );
    const dirs = [...new Set(raw.split('\n').map((l) => l.trim()).filter(Boolean))]
        .filter((d) => d !== FRONTEND_DIR); // drop the frontend package itself
    const entries = [];
    for (const dir of dirs) {
        const pkgJsonPath = join(dir, 'package.json');
        if (!existsSync(pkgJsonPath)) continue;
        let pkg;
        try {
            pkg = JSON.parse(readFileSync(pkgJsonPath, 'utf8'));
        } catch {
            continue;
        }
        const declared = typeof pkg.license === 'string'
            ? pkg.license
            : (pkg.license && pkg.license.type) || (Array.isArray(pkg.licenses) && pkg.licenses[0] && pkg.licenses[0].type) || 'UNKNOWN';
        const text = readLicenseDir(dir);
        entries.push({
            name: pkg.name || dir,
            version: pkg.version || '',
            source: 'npm',
            license: declared,
            text: text || `Declared license: ${declared} (no license file bundled in package).`,
        });
    }
    return entries;
}

function main() {
    console.log('[licenses] collecting Go modules linked into the binary...');
    const go = goEntries();
    console.log(`[licenses]   ${go.length} Go modules`);
    console.log('[licenses] collecting npm production dependencies...');
    const npm = npmEntries();
    console.log(`[licenses]   ${npm.length} npm packages`);

    const entries = [...go, ...npm].sort((a, b) =>
        a.name.toLowerCase().localeCompare(b.name.toLowerCase()));

    // Flag anything copyleft/unknown loudly - these would need review before shipping.
    const flagged = entries.filter((e) =>
        ['AGPL', 'GPL', 'LGPL', 'MPL-2.0', 'EPL', 'CDDL', 'UNKNOWN'].includes(e.license));
    for (const e of flagged) {
        console.warn(`[licenses] REVIEW: ${e.source} ${e.name}@${e.version} classified as ${e.license}`);
    }

    const manifest = {
        note: 'Generated by scripts/gen-third-party-licenses.mjs. Do not edit by hand.',
        count: entries.length,
        entries,
    };
    const json = JSON.stringify(manifest, null, 2) + '\n';

    if (CHECK) {
        const current = existsSync(OUTPUT) ? readFileSync(OUTPUT, 'utf8') : '';
        if (current !== json) {
            console.error('[licenses] STALE: third-party-licenses.json is out of date. Run: node scripts/gen-third-party-licenses.mjs');
            process.exit(1);
        }
        console.log('[licenses] up to date.');
        return;
    }

    writeFileSync(OUTPUT, json);
    console.log(`[licenses] wrote ${entries.length} entries to ${OUTPUT}`);
}

main();
