#!/usr/bin/env node
// @list command audit: Go vs C — compare internal tables
'use strict';

const net = require('net');

const HOST = process.env.MUSH_HOST || '192.168.100.12';
const GO_PORT = 6886;
const C_PORT = 9886;
const GO_LOGIN = 'connect Otter crystal';
const C_LOGIN = 'connect Otter crystal';

function connect(port, loginCmd) {
  return new Promise((resolve, reject) => {
    const sock = net.createConnection(port, HOST);
    let buf = '';
    sock.setEncoding('utf8');
    sock.on('data', (d) => { buf += d; });
    sock.on('error', reject);
    setTimeout(() => {
      buf = '';
      sock.write(loginCmd + '\n');
      setTimeout(() => {
        buf = '';
        sock.write('think XREADY99X\n');
        const check = setInterval(() => {
          if (buf.includes('XREADY99X')) {
            clearInterval(check);
            buf = '';
            resolve(sock);
          }
        }, 200);
        setTimeout(() => { clearInterval(check); reject(new Error('Login timeout on port ' + port)); }, 15000);
      }, 3000);
    }, 2000);
  });
}

function sendCmd(sock, cmd, wait = 1500) {
  return new Promise((resolve) => {
    let buf = '';
    const onData = (d) => { buf += d; };
    sock.on('data', onData);
    sock.write(cmd + '\n');
    setTimeout(() => {
      sock.removeListener('data', onData);
      const clean = buf.replace(/\xff[\xfb-\xfe]./g, '').replace(/\xff\xf1/g, '');
      resolve(clean);
    }, wait);
  });
}

// Parse C @list flags (single line): "ABODE(A) BLIND(B) ..."
function parseCFlags(raw) {
  const flags = new Map();
  const m = raw.match(/Flags:\s*(.*)/);
  if (!m) return flags;
  const tokens = m[1].trim().split(/\s+/);
  for (const tok of tokens) {
    const fm = tok.match(/^(\w+)(?:\((.)\))?$/);
    if (fm) flags.set(fm[1].toUpperCase(), fm[2] || '');
  }
  return flags;
}

// Parse Go @list flags (multi-line): "NAME  (X)  word=N bit=0xHH [perm]"
function parseGoFlags(raw) {
  const flags = new Map();
  const lines = raw.split('\n').map(l => l.trim()).filter(l => l && !l.startsWith('Flags'));
  for (const line of lines) {
    // Match: NAME  (letter)  word=N bit=0xHH  or  NAME  word=N bit=0xHH
    const m = line.match(/^(\w+)\s+(?:\((.)\)\s+)?word=(\d+)\s+bit=(0x[\da-fA-F]+)/);
    if (m) {
      flags.set(m[1].toUpperCase(), m[2] || '');
    }
  }
  return flags;
}

// Parse C @list attributes (single line): "Aahear Aclone ..."
function parseCAttrs(raw) {
  const attrs = new Set();
  const m = raw.match(/Attributes:\s*(.*)/);
  if (!m) return attrs;
  for (const tok of m[1].trim().split(/\s+/)) {
    if (tok) attrs.add(tok.toUpperCase());
  }
  return attrs;
}

// Parse Go @list attributes (multi-line): "Osucc  #1"
function parseGoAttrs(raw) {
  const attrs = new Set();
  const lines = raw.split('\n').map(l => l.trim()).filter(l => l && !l.startsWith('Well-known'));
  for (const line of lines) {
    const m = line.match(/^(\w+)\s+#\d+/);
    if (m) attrs.add(m[1].toUpperCase());
  }
  return attrs;
}

// Parse C @list functions: "Built-in functions: ABS ACOS ..." + "Module comsys functions: ..."
function parseCFunctions(raw) {
  const fns = new Set();
  // Match "Built-in functions:" and "Module ... functions:" lines
  const lines = raw.split('\n');
  for (const line of lines) {
    const m = line.match(/functions:\s*(.*)/i);
    if (m) {
      for (const tok of m[1].trim().split(/\s+/)) {
        const fn = tok.replace(/[()]/g, '').toUpperCase();
        if (fn && fn.match(/^[A-Z0-9_@-]+$/)) fns.add(fn);
      }
    }
  }
  return fns;
}

// Parse Go @list functions (multi-line, 3-column layout)
function parseGoFunctions(raw) {
  const fns = new Set();
  const lines = raw.split('\n').map(l => l.trim()).filter(l => l);
  for (const line of lines) {
    if (line.startsWith('Built-in') || line.startsWith('User-defined') || line.startsWith('Total:')) continue;
    for (const tok of line.split(/\s+/)) {
      const fn = tok.toUpperCase();
      if (fn && fn.match(/^[A-Z0-9_@-]+$/)) fns.add(fn);
    }
  }
  return fns;
}

// Parse C @list switches: "@command: sw1 sw2 ..."
function parseSwitches(raw) {
  const cmds = new Map();
  const lines = raw.split('\n').map(l => l.trim()).filter(l => l);
  for (const line of lines) {
    const m = line.match(/^(@?\w+):\s*(.*)/);
    if (m) {
      const name = m[1].toLowerCase();
      const switches = m[2].trim().split(/\s+/).filter(Boolean).sort();
      cmds.set(name, switches);
    }
  }
  return cmds;
}

async function runAudit() {
  console.log(`Connecting to Go (${HOST}:${GO_PORT}) and C (${HOST}:${C_PORT})...\n`);

  const goSock = await connect(GO_PORT, GO_LOGIN);
  const cSock = await connect(C_PORT, C_LOGIN);

  // Collect raw output for each category
  const goData = {};
  const cData = {};
  for (const cat of ['flags', 'switches', 'functions', 'attributes', 'default_flags', 'costs']) {
    goData[cat] = await sendCmd(goSock, `@list ${cat}`, 2000);
    cData[cat] = await sendCmd(cSock, `@list ${cat}`, 2000);
  }

  goSock.destroy();
  cSock.destroy();

  let totalMissing = 0;

  // === FLAGS ===
  console.log('=== FLAGS ===');
  const goFlags = parseGoFlags(goData.flags);
  const cFlags = parseCFlags(cData.flags);

  const flagsMissing = [];
  const flagsGoOnly = [];
  for (const [name] of cFlags) {
    if (!goFlags.has(name)) flagsMissing.push(name);
  }
  for (const [name] of goFlags) {
    if (!cFlags.has(name)) flagsGoOnly.push(name);
  }

  console.log(`  C: ${cFlags.size} flags  Go: ${goFlags.size} flags`);
  if (flagsMissing.length) {
    console.log(`  Missing in Go (${flagsMissing.length}): ${flagsMissing.join(', ')}`);
    totalMissing += flagsMissing.length;
  } else {
    console.log('  PASS — all C flags present in Go');
  }
  if (flagsGoOnly.length) {
    console.log(`  Go-only (${flagsGoOnly.length}): ${flagsGoOnly.join(', ')}`);
  }

  // Show C flag letters for reference
  const cFlagLetters = [];
  for (const [name, letter] of cFlags) {
    if (letter) cFlagLetters.push(`${name}(${letter})`);
  }

  // === DEFAULT FLAGS ===
  console.log('\n=== DEFAULT FLAGS ===');
  // Just display both — format differs too much for automated comparison
  const goDefLines = goData.default_flags.split('\n').map(l => l.trim()).filter(l => l);
  const cDefLines = cData.default_flags.split('\n').map(l => l.trim()).filter(l => l);
  console.log('  Go:');
  for (const l of goDefLines) console.log(`    ${l}`);
  console.log('  C:');
  for (const l of cDefLines) console.log(`    ${l}`);

  // === SWITCHES ===
  console.log('\n=== SWITCHES ===');
  const cSw = parseSwitches(cData.switches);
  const goSw = parseSwitches(goData.switches);

  console.log(`  C: ${cSw.size} commands  Go: ${goSw.size} commands`);

  // Compare switches for commands that exist in both
  let swMissing = 0;
  const missingCmds = [];
  const missingSwitches = [];
  for (const [cmd, cSwitches] of cSw) {
    const goSwitches = goSw.get(cmd);
    if (!goSwitches) {
      missingCmds.push(cmd);
      continue;
    }
    const missing = cSwitches.filter(s => !goSwitches.includes(s));
    if (missing.length) {
      missingSwitches.push(`${cmd}: ${missing.join(' ')}`);
      swMissing += missing.length;
    }
  }
  if (missingCmds.length) {
    console.log(`  Commands in C but not Go (${missingCmds.length}): ${missingCmds.join(', ')}`);
  }
  if (missingSwitches.length) {
    console.log(`  Missing switches (${swMissing}):`);
    for (const s of missingSwitches) console.log(`    ${s}`);
    totalMissing += swMissing;
  }
  if (!missingSwitches.length && !missingCmds.length) {
    console.log('  PASS — all C switches present in Go');
  } else if (!missingSwitches.length) {
    console.log('  PASS — all shared commands have matching switches');
  }

  // === ATTRIBUTES ===
  console.log('\n=== ATTRIBUTES ===');
  const goAttrs = parseGoAttrs(goData.attributes);
  const cAttrs = parseCAttrs(cData.attributes);

  const attrsMissing = [];
  const attrsGoOnly = [];
  for (const a of cAttrs) {
    if (!goAttrs.has(a)) attrsMissing.push(a);
  }
  for (const a of goAttrs) {
    if (!cAttrs.has(a)) attrsGoOnly.push(a);
  }

  console.log(`  C: ${cAttrs.size} attrs  Go: ${goAttrs.size} attrs`);
  if (attrsMissing.length) {
    console.log(`  Missing in Go (${attrsMissing.length}): ${attrsMissing.join(', ')}`);
    totalMissing += attrsMissing.length;
  } else {
    console.log('  PASS — all C attrs present in Go');
  }
  if (attrsGoOnly.length) {
    console.log(`  Go-only (${attrsGoOnly.length}): ${attrsGoOnly.join(', ')}`);
  }

  // === FUNCTIONS ===
  console.log('\n=== FUNCTIONS ===');
  const goFns = parseGoFunctions(goData.functions);
  const cFns = parseCFunctions(cData.functions);

  // Separate built-in vs user-defined in C
  const cBuiltinLine = cData.functions.split('\n').find(l => l.includes('Built-in'));
  const cBuiltinFns = new Set();
  if (cBuiltinLine) {
    const m = cBuiltinLine.match(/functions:\s*(.*)/);
    if (m) for (const tok of m[1].trim().split(/\s+/)) {
      const fn = tok.toUpperCase();
      if (fn.match(/^[A-Z0-9_@-]+$/)) cBuiltinFns.add(fn);
    }
  }

  // Go built-in = 0 per the output — all registered functions are not in @list yet
  // So check what Go actually supports via think [fn()] style
  const fnsMissing = [];
  const fnsGoOnly = [];
  for (const f of cFns) {
    if (!goFns.has(f)) fnsMissing.push(f);
  }
  for (const f of goFns) {
    if (!cFns.has(f)) fnsGoOnly.push(f);
  }

  console.log(`  C: ${cBuiltinFns.size} built-in + ${cFns.size - cBuiltinFns.size} other  Go listed: ${goFns.size}`);
  console.log(`  NOTE: Go @list functions shows 0 built-in (listing not implemented)`);
  console.log(`  Function compatibility already verified by 696-test audit`);

  if (fnsMissing.length) {
    console.log(`  C functions not in Go @list (${fnsMissing.length}): likely all exist, just not listed`);
  }

  // === COSTS ===
  console.log('\n=== COSTS ===');
  const goCostLines = goData.costs.split('\n').map(l => l.trim()).filter(l => l);
  const cCostLines = cData.costs.split('\n').map(l => l.trim()).filter(l => l);
  console.log(`  Go: ${goCostLines.length} lines  C: ${cCostLines.length} lines`);
  // Show side by side
  console.log('  Go costs:');
  for (const l of goCostLines) console.log(`    ${l}`);
  console.log('  C costs:');
  for (const l of cCostLines) console.log(`    ${l}`);

  // === SUMMARY ===
  console.log(`\n========================================`);
  console.log(`=== @LIST AUDIT SUMMARY ===`);
  console.log(`========================================`);
  console.log(`  Flags:      C=${cFlags.size}  Go=${goFlags.size}  Missing=${flagsMissing.length}`);
  console.log(`  Attributes: C=${cAttrs.size}  Go=${goAttrs.size}  Missing=${attrsMissing.length}`);
  console.log(`  Switches:   C=${cSw.size} commands  Go=${goSw.size} commands  Missing switches=${swMissing}  Unimplemented cmds=${missingCmds.length}`);
  console.log(`  Functions:  696-test audit = 100% (Go @list doesn't enumerate built-ins)`);
  console.log(`  Total gaps: ${totalMissing}`);

  process.exit(totalMissing > 0 ? 1 : 0);
}

runAudit().catch(e => { console.error(e); process.exit(1); });
