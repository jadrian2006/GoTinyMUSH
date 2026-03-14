#!/usr/bin/env node
// Flags audit: permission, visibility, behavior flags — set, check, effect
// Go (192.168.100.12:6886) vs C (192.168.100.12:9886)
// Uses AuditFlags/auditpass account (WIZARD, in own room)
// Uses unique names + captured dbrefs to avoid ambiguity from prior runs
'use strict';

const net = require('net');
const HOST = process.env.MUSH_HOST || '192.168.100.12';
const GO_PORT = 6886;
const C_PORT  = 9886;

function connect(port) {
  return new Promise((resolve, reject) => {
    const sock = net.createConnection(port, HOST);
    sock.setEncoding('utf8');
    sock.on('error', reject);
    setTimeout(() => resolve(sock), 500);
  });
}

// capAll: send multiple commands + marker as ONE write, wait for marker.
// Combines everything to avoid TCP segment reordering / C async issues.
function capAll(sock, cmds, ms = 5000) {
  if (typeof cmds === 'string') cmds = [cmds];
  return new Promise(resolve => {
    let buf = '';
    const marker = 'XFLG' + Date.now() + Math.random().toString(36).slice(2,6) + 'X';
    const onData = d => { buf += d; if (buf.includes(marker)) { sock.off('data', onData); resolve(buf); } };
    sock.on('data', onData);
    // Single write: all commands + marker
    sock.write(cmds.join('\n') + '\n' + `think ${marker}\n`);
    setTimeout(() => { sock.off('data', onData); resolve(buf); }, ms);
  });
}

function lastLine(buf) {
  return buf.replace(/XFLG\S+X/g,'').trim().split('\n')
    .filter(l => {
      const t = l.trim();
      return t && !l.includes('XFLG') && t !== 'Set.' && t !== 'Cleared.'
        && !t.startsWith('Set -') && !t.startsWith('Cleared -')
        && t !== 'drain-ok';
    })
    .pop()?.trim() || '';
}

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

const results = [];
function testExact(name, go1, c1, opts = {}) {
  const g = lastLine(go1);
  const c = lastLine(c1);
  if (opts.cbug) {
    const goOk = opts.goExpect !== undefined ? g === opts.goExpect : true;
    results.push({ name, status: goOk ? 'CBUG' : 'FAIL' });
    console.log(`  ${goOk ? '⚠' : '✗'} ${name} (C 3.1 gap)`);
    if (!goOk) console.log(`    Go: ${g} (expected ${opts.goExpect})`);
    return;
  }
  const pass = g === c;
  results.push({ name, status: pass ? 'PASS' : 'FAIL' });
  console.log(`  ${pass?'✓':'✗'} ${name}`);
  if (!pass) { console.log(`    Go: ${g}`); console.log(`     C: ${c}`); }
}

// Flags that don't exist in C 3.1 — mark as CBUG
const C_MISSING_FLAGS = new Set(['FLOATING']);

(async () => {
  console.log(`=== Flags Audit: Go (:${GO_PORT}) vs C (:${C_PORT}) on ${HOST} ===\n`);

  const goSock = await connect(GO_PORT);
  const cSock  = await connect(C_PORT);
  await sleep(500);
  goSock.write('connect AuditFlags auditpass\n');
  cSock.write('connect AuditFlags auditpass\n');
  await sleep(1000);
  await capAll(goSock, 'think go-ready', 500);
  await capAll(cSock, 'think c-ready', 500);

  // Use unique names to avoid leftover ambiguity
  const uid = Date.now().toString(36);
  const THING1 = `FlgT1${uid}`;
  const THING2 = `FlgT2${uid}`;

  // Create test THING objects — capture dbrefs
  const goT1Ref = lastLine(await capAll(goSock, `think [create(${THING1},10)]`));
  const cT1Ref  = lastLine(await capAll(cSock, `think [create(${THING1},10)]`));
  const goT2Ref = lastLine(await capAll(goSock, `think [create(${THING2},10)]`));
  const cT2Ref  = lastLine(await capAll(cSock, `think [create(${THING2},10)]`));

  console.log(`  Go: ${THING1}=${goT1Ref}, ${THING2}=${goT2Ref}`);
  console.log(`   C: ${THING1}=${cT1Ref}, ${THING2}=${cT2Ref}`);

  if (!goT1Ref.startsWith('#') || !cT1Ref.startsWith('#')) {
    console.log('ERROR: Could not create test objects. Aborting.');
    goSock.destroy(); cSock.destroy(); process.exit(1);
  }

  await sleep(300);

  // Test each flag: set+check and clear+check in single capAll() calls
  // @set + think hasflag sent as one packet — MUSH processes in order
  const FLAGS = [
    'DARK', 'VISUAL', 'OPAQUE', 'PUPPET', 'VERBOSE', 'TRACE', 'STICKY',
    'HAVEN', 'QUIET', 'HALT', 'NOSPOOF', 'SAFE', 'DESTROY_OK',
    'ENTER_OK', 'CHOWN_OK', 'LINK_OK', 'JUMP_OK', 'ABODE', 'FLOATING',
    'UNFINDABLE', 'MYOPIC', 'TERSE', 'AUDIBLE', 'TRANSPARENT',
    'MONITOR', 'ANSI', 'FIXED', 'UNINSPECTED', 'NO_COMMAND',
  ];

  console.log('--- 1: Set/Check/Clear Flags on THING ---');
  for (const flag of FLAGS) {
    const isCBug = C_MISSING_FLAGS.has(flag);

    // Set flag + check hasflag in one atomic operation
    const go1 = await capAll(goSock, [
      `@set ${goT1Ref}=${flag}`,
      `think [hasflag(${goT1Ref},${flag})]`
    ]);
    if (isCBug) {
      testExact(`hasflag ${flag}`, go1, '', { cbug: true, goExpect: '1' });
    } else {
      const c1 = await capAll(cSock, [
        `@set ${cT1Ref}=${flag}`,
        `think [hasflag(${cT1Ref},${flag})]`
      ]);
      testExact(`hasflag ${flag}`, go1, c1);
    }

    // Clear flag + check hasflag in one atomic operation
    const go2 = await capAll(goSock, [
      `@set ${goT1Ref}=!${flag}`,
      `think [hasflag(${goT1Ref},${flag})]`
    ]);
    if (isCBug) {
      testExact(`clear ${flag}`, go2, '', { cbug: true, goExpect: '0' });
    } else {
      const c2 = await capAll(cSock, [
        `@set ${cT1Ref}=!${flag}`,
        `think [hasflag(${cT1Ref},${flag})]`
      ]);
      testExact(`clear ${flag}`, go2, c2);
    }
  }

  console.log('\n--- 2: flags() output format ---');
  {
    await capAll(goSock, [
      `@set ${goT1Ref}=DARK`,
      `@set ${goT1Ref}=VISUAL`,
      `think flags-set-ok`
    ]);
    await capAll(cSock, [
      `@set ${cT1Ref}=DARK`,
      `@set ${cT1Ref}=VISUAL`,
      `think flags-set-ok`
    ]);
    const go1 = await capAll(goSock, `think [flags(${goT1Ref})]`);
    const c1  = await capAll(cSock, `think [flags(${cT1Ref})]`);
    testExact('flags() multi-flag string', go1, c1);
    await capAll(goSock, [
      `@set ${goT1Ref}=!DARK`,
      `@set ${goT1Ref}=!VISUAL`,
      `think flags-cleared`
    ]);
    await capAll(cSock, [
      `@set ${cT1Ref}=!DARK`,
      `@set ${cT1Ref}=!VISUAL`,
      `think flags-cleared`
    ]);
  }

  console.log('\n--- 3: andflags / orflags ---');
  {
    await capAll(goSock, [
      `@set ${goT2Ref}=DARK`,
      `@set ${goT2Ref}=VISUAL`,
      `think set-ok`
    ]);
    await capAll(cSock, [
      `@set ${cT2Ref}=DARK`,
      `@set ${cT2Ref}=VISUAL`,
      `think set-ok`
    ]);

    const go1 = await capAll(goSock, `think [andflags(${goT2Ref},DV)]`);
    const c1  = await capAll(cSock, `think [andflags(${cT2Ref},DV)]`);
    testExact('andflags DV both set', go1, c1);

    const go2 = await capAll(goSock, `think [andflags(${goT2Ref},DH)]`);
    const c2  = await capAll(cSock, `think [andflags(${cT2Ref},DH)]`);
    testExact('andflags DH not both', go2, c2);

    const go3 = await capAll(goSock, `think [orflags(${goT2Ref},DH)]`);
    const c3  = await capAll(cSock, `think [orflags(${cT2Ref},DH)]`);
    testExact('orflags DH one set', go3, c3);

    const go4 = await capAll(goSock, `think [orflags(${goT2Ref},HJ)]`);
    const c4  = await capAll(cSock, `think [orflags(${cT2Ref},HJ)]`);
    testExact('orflags HJ neither set', go4, c4);

    const go5 = await capAll(goSock, `think [andflags(${goT2Ref},!H)]`);
    const c5  = await capAll(cSock, `think [andflags(${cT2Ref},!H)]`);
    testExact('andflags !H (not HAVEN)', go5, c5);
  }

  console.log('\n--- 4: flag effects ---');
  {
    // DARK hides from contents
    await capAll(goSock, [`@set ${goT1Ref}=DARK`, `drop ${goT1Ref}`, `think setup-done`]);
    await capAll(cSock, [`@set ${cT1Ref}=DARK`, `drop ${cT1Ref}`, `think setup-done`]);
    const go1 = await capAll(goSock, 'look');
    const c1  = await capAll(cSock, 'look');
    const goSees = go1.replace(/XFLG\S+X/g,'').includes(THING1);
    const cSees = c1.replace(/XFLG\S+X/g,'').includes(THING1);
    // DARK objects should be hidden from everyone, including wizards
    const pass = !goSees && !cSees;
    results.push({ name: 'DARK wizard visibility', status: pass ? 'PASS' : 'FAIL' });
    console.log(`  ${pass ? '✓' : '✗'} DARK wizard visibility (Go sees: ${goSees}, C sees: ${cSees})`);
    if (!pass) { console.log(`    Expected both false`); }
    await capAll(goSock, [`get ${goT1Ref}`, `@set ${goT1Ref}=!DARK`, `think cleanup-done`]);
    await capAll(cSock, [`get ${cT1Ref}`, `@set ${cT1Ref}=!DARK`, `think cleanup-done`]);
  }
  {
    // ENTER_OK allows enter — verify by checking location
    await capAll(goSock, [`@set ${goT1Ref}=ENTER_OK`, `drop ${goT1Ref}`, `enter ${goT1Ref}`, `think enter-done`]);
    await capAll(cSock, [`@set ${cT1Ref}=ENTER_OK`, `drop ${cT1Ref}`, `enter ${cT1Ref}`, `think enter-done`]);
    const go1 = await capAll(goSock, `think [eq(loc(me),${goT1Ref})]`);
    const c1  = await capAll(cSock, `think [eq(loc(me),${cT1Ref})]`);
    testExact('ENTER_OK allows enter', go1, c1);
    await capAll(goSock, [`leave`, `get ${goT1Ref}`, `@set ${goT1Ref}=!ENTER_OK`, `think leave-done`]);
    await capAll(cSock, [`leave`, `get ${cT1Ref}`, `@set ${cT1Ref}=!ENTER_OK`, `think leave-done`]);
  }

  console.log('\n--- 5: type() with flags ---');
  {
    const go1 = await capAll(goSock, `think [type(${goT1Ref})]`);
    const c1  = await capAll(cSock, `think [type(${cT1Ref})]`);
    testExact('type() THING', go1, c1);
  }

  // ======= CLEANUP =======
  console.log('\n--- Cleanup ---');
  await capAll(goSock, [`@set ${goT1Ref}=DESTROY_OK`, `@destroy ${goT1Ref}`, `think destroyed`]);
  await capAll(goSock, [`@set ${goT2Ref}=DESTROY_OK`, `@destroy ${goT2Ref}`, `think destroyed`]);
  await capAll(cSock, [`@set ${cT1Ref}=DESTROY_OK`, `@destroy ${cT1Ref}`, `think destroyed`]);
  await capAll(cSock, [`@set ${cT2Ref}=DESTROY_OK`, `@destroy ${cT2Ref}`, `think destroyed`]);
  await sleep(300);

  // Summary
  console.log('\n=== Summary ===');
  const pass = results.filter(r => r.status === 'PASS').length;
  const fail = results.filter(r => r.status === 'FAIL').length;
  const cbug = results.filter(r => r.status === 'CBUG').length;
  console.log(`Total: ${results.length} | Pass: ${pass} | Fail: ${fail} | C Bug: ${cbug}`);
  for (const r of results) {
    const icon = r.status === 'PASS' ? '✓' : r.status === 'CBUG' ? '⚠' : '✗';
    console.log(`  ${icon} ${r.status}: ${r.name}`);
  }
  console.log('\nDone.');
  goSock.destroy();
  cSock.destroy();
  process.exit(fail > 0 ? 1 : 0);
})();
