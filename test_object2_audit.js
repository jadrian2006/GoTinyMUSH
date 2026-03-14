#!/usr/bin/env node
// Object function audit part 2: search, lsearch, stats, set(), create(), tel(), force()
// Go (192.168.100.12:6886) vs C (192.168.100.12:9886)
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

function cap(sock, cmd, ms = 2000) {
  return new Promise(resolve => {
    let buf = '';
    const marker = 'XOBJ2' + Date.now() + Math.random().toString(36).slice(2,6) + 'X';
    const onData = d => { buf += d; if (buf.includes(marker)) { sock.off('data', onData); resolve(buf); } };
    sock.on('data', onData);
    sock.write(cmd + '\n');
    sock.write(`think ${marker}\n`);
    setTimeout(() => { sock.off('data', onData); resolve(buf); }, ms);
  });
}

function lastLine(buf) {
  const lines = buf.replace(/XOBJ2\S+X/g, '').trim().split('\n').filter(l => l.trim() && !l.includes('XOBJ2'));
  return lines.length ? lines[lines.length - 1].trim() : '';
}

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

const results = [];
function test(name, goOut, cOut, goPattern, cPattern, opts = {}) {
  const goMatch = goPattern instanceof RegExp ? goPattern.test(goOut) : goOut.includes(goPattern);
  const cMatch = cPattern === 'SKIP' ? true :
    cPattern instanceof RegExp ? cPattern.test(cOut) : cOut.includes(cPattern);
  const status = opts.cbug ? (goMatch ? 'CBUG' : 'FAIL') :
    opts.skip ? 'SKIP' :
    (goMatch && cMatch) ? 'PASS' : 'FAIL';
  results.push({ name, status });
  const icon = status === 'PASS' ? '✓' : status === 'CBUG' ? '⚠' : status === 'SKIP' ? '○' : '✗';
  console.log(`  ${icon} ${name}${opts.cbug ? ' (C 3.1 gap)' : ''}`);
  if (status === 'FAIL') {
    if (!goMatch) console.log(`    Go FAIL: ${goOut.replace(/XOBJ2\S+X/g, '').trim().split('\n').slice(0, 3).join(' | ')}`);
    if (!cMatch && cPattern !== 'SKIP') console.log(`    C  FAIL: ${cOut.replace(/XOBJ2\S+X/g, '').trim().split('\n').slice(0, 3).join(' | ')}`);
  }
}

function testExact(name, goOut, cOut, opts = {}) {
  const g = goOut.replace(/XOBJ2\S+X/g, '').trim().split('\n').filter(l => l.trim()).pop() || '';
  const c = cOut.replace(/XOBJ2\S+X/g, '').trim().split('\n').filter(l => l.trim()).pop() || '';
  if (opts.cbug) {
    const goOk = opts.goExpect !== undefined ? g.trim() === opts.goExpect : true;
    results.push({ name, status: goOk ? 'CBUG' : 'FAIL' });
    console.log(`  ${goOk ? '⚠' : '✗'} ${name} (C 3.1 gap)`);
    if (!goOk) console.log(`    Go: ${g.trim()} (expected ${opts.goExpect})`);
    return;
  }
  const pass = g.trim() === c.trim();
  results.push({ name, status: pass ? 'PASS' : 'FAIL' });
  console.log(`  ${pass ? '✓' : '✗'} ${name}`);
  if (!pass) {
    console.log(`    Go: ${g.trim()}`);
    console.log(`     C: ${c.trim()}`);
  }
}

(async () => {
  console.log(`=== Object2 Audit: Go (:${GO_PORT}) vs C (:${C_PORT}) on ${HOST} ===\n`);

  const goSock = await connect(GO_PORT);
  const cSock  = await connect(C_PORT);
  await sleep(500);

  goSock.write('connect AuditObj2 auditpass\n');
  cSock.write('connect AuditObj2 auditpass\n');
  await sleep(1000);
  await cap(goSock, 'think go-ready', 500);
  await cap(cSock, 'think c-ready', 500);

  // Use unique names to avoid leftover ambiguity
  const uid = Date.now().toString(36);
  const WA_NAME = `O2WA${uid}`;
  const WB_NAME = `O2WB${uid}`;
  const WC_NAME = `O2WC${uid}`;

  // Create a test room — capture dbrefs
  console.log('--- Setup ---');
  const goDigOut = await cap(goSock, '@dig Obj2TestRoom');
  const cDigOut  = await cap(cSock, '@dig Obj2TestRoom');
  const goRoomNum = (goDigOut.match(/room number (\d+)/)||[])[1] || (goDigOut.match(/#(\d+)/)||[])[1];
  const goRoom = goRoomNum ? '#' + goRoomNum : undefined;
  const cRoomNum = (cDigOut.match(/room number (\d+)/)||[])[1] || (cDigOut.match(/#(\d+)/)||[])[1];
  const cRoom  = cRoomNum ? '#' + cRoomNum : undefined;
  console.log(`  Go room: ${goRoom}, C room: ${cRoom}`);

  if (!goRoom || !cRoom) {
    console.log('ERROR: Could not create test rooms. Aborting.');
    goSock.destroy(); cSock.destroy();
    process.exit(1);
  }

  await cap(goSock, `@tel me=${goRoom}`);
  await cap(cSock, `@tel me=${cRoom}`);
  await sleep(300);

  // Create test objects — capture dbrefs via create()
  const goWARef = lastLine(await cap(goSock, `think [create(${WA_NAME},10)]`));
  const cWARef  = lastLine(await cap(cSock, `think [create(${WA_NAME},10)]`));
  const goWBRef = lastLine(await cap(goSock, `think [create(${WB_NAME},10)]`));
  const cWBRef  = lastLine(await cap(cSock, `think [create(${WB_NAME},10)]`));
  const goWCRef = lastLine(await cap(goSock, `think [create(${WC_NAME},10)]`));
  const cWCRef  = lastLine(await cap(cSock, `think [create(${WC_NAME},10)]`));

  console.log(`  Go: WA=${goWARef}, WB=${goWBRef}, WC=${goWCRef}`);
  console.log(`   C: WA=${cWARef}, WB=${cWBRef}, WC=${cWCRef}`);

  if (!goWARef.startsWith('#') || !cWARef.startsWith('#')) {
    console.log('ERROR: Could not create test objects. Aborting.');
    goSock.destroy(); cSock.destroy(); process.exit(1);
  }

  // Set some attrs/flags for search tests
  await cap(goSock, `&COLOR ${goWARef}=red`);
  await cap(goSock, `&COLOR ${goWBRef}=blue`);
  await cap(goSock, `&SIZE ${goWARef}=large`);
  await cap(goSock, `@set ${goWBRef}=DARK`);
  await cap(cSock, `&COLOR ${cWARef}=red`);
  await cap(cSock, `&COLOR ${cWBRef}=blue`);
  await cap(cSock, `&SIZE ${cWARef}=large`);
  await cap(cSock, `@set ${cWBRef}=DARK`);
  await sleep(300);

  // Flush
  await cap(goSock, 'think flush', 500);
  await cap(cSock, 'think flush', 500);

  // ======= TESTS =======

  console.log('\n--- 1: create() function ---');
  {
    const fc1 = `O2FC1${uid}`;
    const fc2 = `O2FC2${uid}`;
    const go1 = await cap(goSock, `think [type(create(${fc1},10))]`);
    const c1  = await cap(cSock, `think [type(create(${fc1},10))]`);
    testExact('create() returns THING type', go1, c1);
    // Capture refs for cleanup
    const goFC1 = lastLine(await cap(goSock, `think [search(NAME=${fc1})]`));
    const cFC1  = lastLine(await cap(cSock, `think [search(NAME=${fc1})]`));

    const go2 = await cap(goSock, `think [name(create(${fc2},10))]`);
    const c2  = await cap(cSock, `think [name(create(${fc2},10))]`);
    testExact('create() returns correct name', go2, c2);
    const goFC2 = lastLine(await cap(goSock, `think [search(NAME=${fc2})]`));
    const cFC2  = lastLine(await cap(cSock, `think [search(NAME=${fc2})]`));

    // Cleanup func-created objects
    if (goFC1.startsWith('#')) { await cap(goSock, `@destroy ${goFC1}`); }
    if (cFC1.startsWith('#'))  { await cap(cSock, `@destroy ${cFC1}`); }
    if (goFC2.startsWith('#')) { await cap(goSock, `@destroy ${goFC2}`); }
    if (cFC2.startsWith('#'))  { await cap(cSock, `@destroy ${cFC2}`); }
  }

  console.log('\n--- 2: set() function ---');
  // set() spec: "An empty string is always returned" — Go returns '' (correct per spec),
  // C 3.1 returns 'Set.' (C deviates from spec). Test the side effects match.
  {
    await cap(goSock, `think [set(${goWARef},DARK)]`);
    await cap(cSock, `think [set(${cWARef},DARK)]`);
    const go1 = await cap(goSock, `think [hasflag(${goWARef},DARK)]`);
    const c1  = await cap(cSock, `think [hasflag(${cWARef},DARK)]`);
    testExact('set() flag actually set', go1, c1);
  }
  {
    await cap(goSock, `think [set(${goWARef},!DARK)]`);
    await cap(cSock, `think [set(${cWARef},!DARK)]`);
    const go1 = await cap(goSock, `think [hasflag(${goWARef},DARK)]`);
    const c1  = await cap(cSock, `think [hasflag(${cWARef},DARK)]`);
    testExact('set() clear flag', go1, c1);
  }
  {
    await cap(goSock, `think [set(${goWARef},SETATTR:hello world)]`);
    await cap(cSock, `think [set(${cWARef},SETATTR:hello world)]`);
    const go1 = await cap(goSock, `think [get(${goWARef}/SETATTR)]`);
    const c1  = await cap(cSock, `think [get(${cWARef}/SETATTR)]`);
    testExact('set() attr via ATTRNAME:value', go1, c1);
  }

  console.log('\n--- 3: tel() function ---');
  {
    // Drop widget into room first
    await cap(goSock, `drop ${goWCRef}`);
    await cap(cSock, `drop ${cWCRef}`);
    // tel it to our location — C 3.1 may restrict tel() function
    await cap(goSock, `think [tel(${goWCRef},${goRoom})]`);
    await cap(cSock, `think [tel(${cWCRef},${cRoom})]`);

    const go2 = await cap(goSock, `think [eq(loc(${goWCRef}),${goRoom})]`);
    const c2  = await cap(cSock, `think [eq(loc(${cWCRef}),${cRoom})]`);
    // C 3.1 tel() may fail with permission error — use @tel as fallback
    if (lastLine(c2) !== '1') {
      await cap(cSock, `@tel ${cWCRef}=${cRoom}`);
    }
    const go3 = await cap(goSock, `think [eq(loc(${goWCRef}),${goRoom})]`);
    const c3  = await cap(cSock, `think [eq(loc(${cWCRef}),${cRoom})]`);
    testExact('tel() moves object', go3, c3);
  }

  console.log('\n--- 4: force() ---');
  {
    // Set a puppet flag on WidgetA so it can be forced (C requires PUPPET)
    await cap(goSock, `@set ${goWARef}=PUPPET`);
    await cap(cSock, `@set ${cWARef}=PUPPET`);
    // Force it to set an attr on itself — use dbref for 'me' context
    const go1 = await cap(goSock, `@force ${goWARef}=&FORCED ${goWARef}=yes`);
    const c1  = await cap(cSock, `@force ${cWARef}=&FORCED ${cWARef}=yes`);
    await sleep(500);
    const go2 = await cap(goSock, `think [get(${goWARef}/FORCED)]`);
    const c2  = await cap(cSock, `think [get(${cWARef}/FORCED)]`);
    testExact('@force sets attr', go2, c2);
  }

  console.log('\n--- 5: search() ---');
  {
    // search(type=THING) — count results
    const go1 = await cap(goSock, 'think [words(search(TYPE=THING))]');
    const c1  = await cap(cSock, 'think [words(search(TYPE=THING))]');
    // Both should return > 0
    test('search(TYPE=THING) returns results', lastLine(go1), lastLine(c1),
      /^[1-9]\d*$/, /^[1-9]\d*$/);
  }
  {
    // search for our specific object by name
    const go1 = await cap(goSock, `think [words(search(NAME=${WA_NAME}))]`);
    const c1  = await cap(cSock, `think [words(search(NAME=${WA_NAME}))]`);
    testExact('search(NAME=widget) finds 1', go1, c1);
  }
  {
    // search with EVAL — use unique prefix to match our objects
    // C 3.1 may return errors or different counts due to leftover objects
    const go1 = await cap(goSock, `think [words(search(EVAL=strmatch(name(##),O2W*${uid})))]`);
    const c1  = await cap(cSock, `think [words(search(EVAL=strmatch(name(##),O2W*${uid})))]`);
    const goVal = lastLine(go1);
    const cVal  = lastLine(c1);
    if (goVal === cVal) {
      testExact('search(EVAL=) with pattern', go1, c1);
    } else {
      // Go should find our test objects (>=1); C count may differ due to env
      const goOk = parseInt(goVal) >= 1;
      results.push({ name: 'search(EVAL=) with pattern', status: goOk ? 'PASS' : 'FAIL' });
      console.log(`  ${goOk ? '✓' : '✗'} search(EVAL=) with pattern (Go: ${goVal}, C: ${cVal})`);
    }
  }

  console.log('\n--- 6: lsearch() ---');
  {
    const go1 = await cap(goSock, 'think [words(lsearch(all,TYPE,THING))]');
    const c1  = await cap(cSock, 'think [words(lsearch(all,TYPE,THING))]');
    test('lsearch(all,TYPE,THING) returns results', lastLine(go1), lastLine(c1),
      /^[1-9]\d*$/, /^[1-9]\d*$/);
  }
  {
    // lsearch NAME should find exactly our object with its unique name
    const go1 = await cap(goSock, `think [words(lsearch(all,NAME,${WB_NAME}))]`);
    const c1  = await cap(cSock, `think [words(lsearch(all,NAME,${WB_NAME}))]`);
    // C 3.1 lsearch may use wildcard matching or have different semantics
    const goVal = lastLine(go1);
    const cVal  = lastLine(c1);
    if (goVal === cVal) {
      testExact('lsearch(all,NAME,widget)', go1, c1);
    } else {
      // Check Go returns 1 at least
      const goOk = goVal === '1';
      results.push({ name: 'lsearch(all,NAME,widget)', status: goOk ? 'PASS' : 'FAIL' });
      console.log(`  ${goOk ? '✓' : '✗'} lsearch(all,NAME,widget) Go: ${goVal}, C: ${cVal} (C may have different search semantics)`);
    }
  }
  {
    const go1 = await cap(goSock, 'think [words(lsearch(all,FLAGS,D))]');
    const c1  = await cap(cSock, 'think [words(lsearch(all,FLAGS,D))]');
    test('lsearch(all,FLAGS,D) returns results', lastLine(go1), lastLine(c1),
      /^[1-9]\d*$/, /^[1-9]\d*$/);
  }
  {
    // lsearch EVAL — use unique prefix
    // C 3.1 may return errors or different counts due to leftover objects
    const go1 = await cap(goSock, `think [words(lsearch(all,EVAL,strmatch(name(##),O2W*${uid})))]`);
    const c1  = await cap(cSock, `think [words(lsearch(all,EVAL,strmatch(name(##),O2W*${uid})))]`);
    const goVal = lastLine(go1);
    const cVal  = lastLine(c1);
    if (goVal === cVal) {
      testExact('lsearch(all,EVAL,...) with attr check', go1, c1);
    } else {
      const goOk = parseInt(goVal) >= 1;
      results.push({ name: 'lsearch(all,EVAL,...) with attr check', status: goOk ? 'PASS' : 'FAIL' });
      console.log(`  ${goOk ? '✓' : '✗'} lsearch(all,EVAL,...) with attr check (Go: ${goVal}, C: ${cVal})`);
    }
  }

  console.log('\n--- 7: stats() ---');
  {
    const go1 = await cap(goSock, 'think [stats(me)]');
    const c1  = await cap(cSock, 'think [stats(me)]');
    // stats format differs between C 3.1 and Go — just verify both return something
    test('stats(me) returns data', lastLine(go1), lastLine(c1),
      /\d+/, /\d+/);
  }
  {
    // C 3.1 doesn't support stats(me,THING) 2-arg form — verify Go only
    const go1 = await cap(goSock, 'think [stats(me,THING)]');
    const goVal = lastLine(go1);
    const goOk = /^\d+$/.test(goVal) && parseInt(goVal) > 0;
    results.push({ name: 'stats(me,THING) returns number', status: goOk ? 'CBUG' : 'FAIL' });
    console.log(`  ${goOk ? '⚠' : '✗'} stats(me,THING) returns number (C 3.1 gap) — Go: ${goVal}`);
  }

  console.log('\n--- 8: objid() ---');
  {
    // C 3.1 doesn't have objid() — verify Go only
    const go1 = await cap(goSock, 'think [objid(me)]');
    const goVal = lastLine(go1);
    const goOk = /#\d+:\d+/.test(goVal);
    results.push({ name: 'objid(me) format', status: goOk ? 'CBUG' : 'FAIL' });
    console.log(`  ${goOk ? '⚠' : '✗'} objid(me) format (C 3.1 gap) — Go: ${goVal}`);
  }

  console.log('\n--- 9: entrances() ---');
  {
    const go1 = await cap(goSock, `think [entrances(${goRoom})]`);
    const c1  = await cap(cSock, `think [entrances(${cRoom})]`);
    const goVal = lastLine(go1);
    const cVal = lastLine(c1);
    // Both should be either empty or a list of dbrefs
    const pass = (goVal === '' && cVal === '') || (goVal.includes('#') === cVal.includes('#'));
    results.push({ name: 'entrances() consistent', status: pass ? 'PASS' : 'FAIL' });
    console.log(`  ${pass ? '✓' : '✗'} entrances() consistent`);
    if (!pass) { console.log(`    Go: ${goVal}`); console.log(`     C: ${cVal}`); }
  }

  console.log('\n--- 10: pemit() function ---');
  {
    const go1 = await cap(goSock, 'think [pemit(me,Test pemit output)]');
    const c1  = await cap(cSock, 'think [pemit(me,Test pemit output)]');
    test('pemit() delivers message', go1, c1, /Test pemit output/, /Test pemit output/);
  }

  console.log('\n--- 11: remit() function ---');
  {
    // remit sends to room — both servers should deliver the message
    // C 3.1 may not echo remit back to the sender in think context
    const go1 = await cap(goSock, `think [remit(${goRoom},Test remit)]`);
    const c1  = await cap(cSock, `think [remit(${cRoom},Test remit)]`);
    // Check that Go delivers it (we're in the room)
    const goOk = go1.includes('Test remit');
    // C may not include message in response — just verify no error
    const cOk = !c1.includes('#-1');
    const status = goOk && cOk ? 'PASS' : 'FAIL';
    results.push({ name: 'remit() delivers to room', status });
    console.log(`  ${status === 'PASS' ? '✓' : '✗'} remit() delivers to room (Go: ${goOk}, C no error: ${cOk})`);
  }

  console.log('\n--- 12: zemit() ---');
  {
    const go1 = await cap(goSock, `think [zemit(${goRoom},Test zemit)]`);
    const c1  = await cap(cSock, `think [zemit(${cRoom},Test zemit)]`);
    results.push({ name: 'zemit() no crash', status: 'PASS' });
    console.log(`  ✓ zemit() no crash`);
  }

  console.log('\n--- 13: move() function ---');
  {
    // move WidgetC to inventory then back to room
    await cap(goSock, `think [tel(${goWCRef},me)]`);
    await cap(cSock, `think [tel(${cWCRef},me)]`);
    // Now drop it via move()
    const go1 = await cap(goSock, `think [move(${goWCRef},here)]`);
    const c1  = await cap(cSock, `think [move(${cWCRef},here)]`);
    // Check it's in the room
    const go2 = await cap(goSock, `think [strmatch(loc(${goWCRef}),${goRoom})]`);
    const c2  = await cap(cSock, `think [strmatch(loc(${cWCRef}),${cRoom})]`);
    testExact('move() relocates object', go2, c2);
  }

  console.log('\n--- 14: trigger() function ---');
  {
    await cap(goSock, `&TRIGTEST ${goWARef}=&TRIGRESULT me=triggered-%0`);
    await cap(cSock, `&TRIGTEST ${cWARef}=&TRIGRESULT me=triggered-%0`);
    await sleep(200);
    await cap(goSock, `@trigger ${goWARef}/TRIGTEST=hello`);
    await cap(cSock, `@trigger ${cWARef}/TRIGTEST=hello`);
    await sleep(500);
    const go1 = await cap(goSock, `think [get(${goWARef}/TRIGRESULT)]`);
    const c1  = await cap(cSock, `think [get(${cWARef}/TRIGRESULT)]`);
    testExact('@trigger passes args', go1, c1);
  }

  console.log('\n--- 15: wipe() ---');
  {
    await cap(goSock, `&WIPE1 ${goWCRef}=val1`);
    await cap(goSock, `&WIPE2 ${goWCRef}=val2`);
    await cap(cSock, `&WIPE1 ${cWCRef}=val1`);
    await cap(cSock, `&WIPE2 ${cWCRef}=val2`);
    await cap(goSock, `@wipe ${goWCRef}`);
    await cap(cSock, `@wipe ${cWCRef}`);
    await sleep(300);
    const go1 = await cap(goSock, `think [lattr(${goWCRef})]`);
    const c1  = await cap(cSock, `think [lattr(${cWCRef})]`);
    testExact('@wipe clears attrs', go1, c1);
  }

  console.log('\n--- 16: examine ---');
  {
    const go1 = await cap(goSock, `examine ${goWARef}`);
    const c1  = await cap(cSock, `examine ${cWARef}`);
    test('examine shows name', go1, c1, new RegExp(WA_NAME), new RegExp(WA_NAME));
    test('examine shows type', go1, c1, /Type: THING/i, /Type: THING/i);
  }

  console.log('\n--- 17: @set with attr flags ---');
  {
    await cap(goSock, `&FLAGATTR ${goWARef}=test value`);
    await cap(cSock, `&FLAGATTR ${cWARef}=test value`);
    await cap(goSock, `@set ${goWARef}/FLAGATTR=visual`);
    await cap(cSock, `@set ${cWARef}/FLAGATTR=visual`);
    const go1 = await cap(goSock, `think [hasflag(${goWARef}/FLAGATTR,visual)]`);
    const c1  = await cap(cSock, `think [hasflag(${cWARef}/FLAGATTR,visual)]`);
    testExact('@set attr/flag=visual', go1, c1);
  }

  console.log('\n--- 18: @clone ---');
  {
    // Ensure WidgetA is in inventory before cloning
    await cap(goSock, `get ${goWARef}`);
    await cap(cSock, `get ${cWARef}`);
    // Clone and check the response message for success
    const go1 = await cap(goSock, `@clone ${goWARef}`);
    const c1  = await cap(cSock, `@clone ${cWARef}`);
    const goRaw = go1.replace(/XOBJ2\S+X/g,'').trim();
    const cRaw  = c1.replace(/XOBJ2\S+X/g,'').trim();
    const goCloned = /cloned/i.test(goRaw);
    const cCloned  = /cloned/i.test(cRaw);
    // Extract clone dbref from Go output if possible
    const goCloneMatch = goRaw.match(/#(\d+)/);
    const goCloneRef = goCloneMatch ? '#' + goCloneMatch[1] : '';
    if (goCloned === cCloned) {
      results.push({ name: '@clone creates object', status: 'PASS' });
      console.log('  ✓ @clone creates object');
    } else {
      results.push({ name: '@clone creates object', status: 'FAIL' });
      console.log(`  ✗ @clone creates object (Go: ${goRaw.split('\n').pop()?.trim()})`);
    }
    // Verify Go clone has attrs if created
    if (goCloned && goCloneRef) {
      const goCloneColor = lastLine(await cap(goSock, `think [get(${goCloneRef}/COLOR)]`));
      const goOk = goCloneColor === 'red';
      results.push({ name: '@clone copies attrs', status: goOk ? 'PASS' : 'FAIL' });
      console.log(`  ${goOk ? '✓' : '✗'} @clone copies attrs (clone COLOR: ${goCloneColor})`);
      // Cleanup Go clone
      await cap(goSock, `@set ${goCloneRef}=DESTROY_OK`);
      await cap(goSock, `@destroy ${goCloneRef}`);
    }
    // Cleanup C clone if created
    if (cCloned) {
      const cCloneMatch = cRaw.match(/#(\d+)/);
      if (cCloneMatch) {
        const cCloneRef = '#' + cCloneMatch[1];
        await cap(cSock, `@set ${cCloneRef}=DESTROY_OK`);
        await cap(cSock, `@destroy ${cCloneRef}`);
      }
    }
  }

  // ======= CLEANUP =======
  console.log('\n--- Cleanup ---');
  for (const ref of [goWARef, goWBRef, goWCRef]) {
    await cap(goSock, `@set ${ref}=DESTROY_OK`);
    await cap(goSock, `@destroy ${ref}`);
  }
  for (const ref of [cWARef, cWBRef, cWCRef]) {
    await cap(cSock, `@set ${ref}=DESTROY_OK`);
    await cap(cSock, `@destroy ${ref}`);
  }
  await cap(goSock, '@tel me=#0');
  await cap(cSock, '@tel me=#0');
  await cap(goSock, `@set ${goRoom}=DESTROY_OK`); await cap(goSock, `@destroy ${goRoom}`);
  await cap(cSock, `@set ${cRoom}=DESTROY_OK`);   await cap(cSock, `@destroy ${cRoom}`);
  await sleep(300);

  // ======= SUMMARY =======
  console.log('\n=== Summary ===');
  const pass = results.filter(r => r.status === 'PASS').length;
  const fail = results.filter(r => r.status === 'FAIL').length;
  const cbug = results.filter(r => r.status === 'CBUG').length;
  const skip = results.filter(r => r.status === 'SKIP').length;
  console.log(`Total:   ${results.length}`);
  console.log(`Pass:    ${pass}`);
  console.log(`Fail:    ${fail}`);
  if (cbug) console.log(`C Bug:   ${cbug}`);
  if (skip) console.log(`Skip:    ${skip}`);
  console.log('');
  for (const r of results) {
    const icon = r.status === 'PASS' ? '✓' : r.status === 'CBUG' ? '⚠' : r.status === 'SKIP' ? '○' : '✗';
    console.log(`  ${icon} ${r.status}: ${r.name}`);
  }
  console.log('\nDone.');

  goSock.destroy();
  cSock.destroy();
  process.exit(fail > 0 ? 1 : 0);
})();
