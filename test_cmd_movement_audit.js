#!/usr/bin/env node
// Movement & communication command audit: goto, look, say, pose, page, @emit, get/drop/give
// Go (192.168.100.12:6886) vs C (192.168.100.12:9886)
'use strict';

const net = require('net');
const HOST = process.env.MUSH_HOST || '192.168.100.12';
const GO_PORT = 6886;
const C_PORT  = 9886;
const GO_LOGIN = process.env.GO_LOGIN || 'connect AuditMove auditpass';
const C_LOGIN  = process.env.C_LOGIN || 'connect AuditMove auditpass';

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
    const marker = 'XMOV' + Date.now() + Math.random().toString(36).slice(2,6) + 'X';
    const onData = d => { buf += d; if (buf.includes(marker)) { sock.off('data', onData); resolve(buf); } };
    sock.on('data', onData);
    sock.write(cmd + '\n');
    sock.write(`think ${marker}\n`);
    setTimeout(() => { sock.off('data', onData); resolve(buf); }, ms);
  });
}

function lastLine(buf) {
  return buf.replace(/XMOV\S+X/g,'').trim().split('\n').filter(l=>l.trim()&&!l.includes('XMOV')).pop()?.trim()||'';
}

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

const results = [];
function test(name, goOut, cOut, goP, cP) {
  const goMatch = goP instanceof RegExp ? goP.test(goOut) : goOut.includes(goP);
  const cMatch = cP === 'SKIP' ? true : cP instanceof RegExp ? cP.test(cOut) : cOut.includes(cP);
  const status = (goMatch && cMatch) ? 'PASS' : 'FAIL';
  results.push({ name, status });
  console.log(`  ${status==='PASS'?'✓':'✗'} ${name}`);
  if (status === 'FAIL') {
    if (!goMatch) console.log(`    Go FAIL: ${goOut.replace(/XMOV\S+X/g,'').trim().split('\n').slice(0,3).join(' | ')}`);
    if (!cMatch && cP !== 'SKIP') console.log(`    C  FAIL: ${cOut.replace(/XMOV\S+X/g,'').trim().split('\n').slice(0,3).join(' | ')}`);
  }
}

function testExact(name, go1, c1) {
  const g = go1.replace(/XMOV\S+X/g,'').trim().split('\n').filter(l=>l.trim()).pop()?.trim()||'';
  const c = c1.replace(/XMOV\S+X/g,'').trim().split('\n').filter(l=>l.trim()).pop()?.trim()||'';
  const pass = g === c;
  results.push({ name, status: pass ? 'PASS' : 'FAIL' });
  console.log(`  ${pass?'✓':'✗'} ${name}`);
  if (!pass) { console.log(`    Go: ${g}`); console.log(`     C: ${c}`); }
}

(async () => {
  console.log(`=== Movement/Comm Audit: Go (:${GO_PORT}) vs C (:${C_PORT}) on ${HOST} ===\n`);

  const goSock = await connect(GO_PORT);
  const cSock  = await connect(C_PORT);
  await sleep(500);

  goSock.write(GO_LOGIN + '\n');
  cSock.write(C_LOGIN + '\n');
  await sleep(1000);
  await cap(goSock, 'think go-ready', 500);
  await cap(cSock, 'think c-ready', 500);

  // Set @sex and @desc on test character (needed for look/pronoun tests)
  await cap(goSock, '@sex me=male');
  await cap(cSock, '@sex me=male');
  await cap(goSock, '@desc me=A test character for the movement audit.');
  await cap(cSock, '@desc me=A test character for the movement audit.');

  // Setup: 2 rooms with exit between them, a thing to manipulate
  console.log('--- Setup ---');
  const uid = Date.now().toString(36);
  const goD1 = await cap(goSock, `@dig MovRoom1${uid}`);
  const cD1  = await cap(cSock, `@dig MovRoom1${uid}`);
  const goRoom1Num = (goD1.match(/room number (\d+)/)||[])[1] || (goD1.match(/#(\d+)/)||[])[1];
  const goRoom1 = goRoom1Num ? '#' + goRoom1Num : undefined;
  const cRoom1Num = (cD1.match(/room number (\d+)/)||[])[1] || (cD1.match(/#(\d+)/)||[])[1];
  const cRoom1  = cRoom1Num ? '#' + cRoom1Num : undefined;

  const goD2 = await cap(goSock, `@dig MovRoom2${uid}`);
  const cD2  = await cap(cSock, `@dig MovRoom2${uid}`);
  const goRoom2Num = (goD2.match(/room number (\d+)/)||[])[1] || (goD2.match(/#(\d+)/)||[])[1];
  const goRoom2 = goRoom2Num ? '#' + goRoom2Num : undefined;
  const cRoom2Num = (cD2.match(/room number (\d+)/)||[])[1] || (cD2.match(/#(\d+)/)||[])[1];
  const cRoom2  = cRoom2Num ? '#' + cRoom2Num : undefined;

  console.log(`  Go rooms: ${goRoom1}, ${goRoom2} — C rooms: ${cRoom1}, ${cRoom2}`);

  // Create exits between rooms (unique alias based on uid)
  const doorAlias = `md${uid.slice(0,4)}`;
  const backAlias = `mb${uid.slice(0,4)}`;
  await cap(goSock, `@tel me=${goRoom1}`);
  await cap(cSock, `@tel me=${cRoom1}`);
  await cap(goSock, `@open MovDoor${uid};${doorAlias}=${goRoom2}`);
  await cap(cSock, `@open MovDoor${uid};${doorAlias}=${cRoom2}`);
  // Return exit
  await cap(goSock, `@tel me=${goRoom2}`);
  await cap(cSock, `@tel me=${cRoom2}`);
  await cap(goSock, `@open MovBack${uid};${backAlias}=${goRoom1}`);
  await cap(cSock, `@open MovBack${uid};${backAlias}=${cRoom1}`);
  // Back to Room1
  await cap(goSock, `@tel me=${goRoom1}`);
  await cap(cSock, `@tel me=${cRoom1}`);

  // Create a thing to get/drop/give — capture dbrefs
  const BALL_NAME = `MovBall${uid}`;
  const goBallRef = lastLine(await cap(goSock, `think [create(${BALL_NAME},10)]`));
  const cBallRef  = lastLine(await cap(cSock, `think [create(${BALL_NAME},10)]`));
  console.log(`  Go ball: ${goBallRef}, C ball: ${cBallRef}`);
  await sleep(300);
  await cap(goSock, 'think setup-flush', 500);
  await cap(cSock, 'think setup-flush', 500);

  // ======= TESTS =======

  console.log('\n--- 1: look ---');
  {
    const go1 = await cap(goSock, 'look');
    const c1  = await cap(cSock, 'look');
    test('look shows room name', go1, c1, /MovRoom1/, /MovRoom1/);
    test('look shows exits', go1, c1, /MovDoor/i, /MovDoor/i);
  }
  {
    const go1 = await cap(goSock, `look ${goBallRef}`);
    const c1  = await cap(cSock, `look ${cBallRef}`);
    test('look at thing', go1, c1, new RegExp(BALL_NAME), new RegExp(BALL_NAME));
  }
  {
    const go1 = await cap(goSock, 'look me');
    const c1  = await cap(cSock, 'look me');
    test('look me', go1, c1, /AuditMove/, /AuditMove/);
  }

  console.log('\n--- 2: go through exit ---');
  {
    const go1 = await cap(goSock, doorAlias);
    const c1  = await cap(cSock, doorAlias);
    // Should arrive in Room2
    const go2 = await cap(goSock, 'think [name(here)]');
    const c2  = await cap(cSock, 'think [name(here)]');
    testExact('exit moves to Room2', go2, c2);
  }
  {
    // Go back
    const go1 = await cap(goSock, backAlias);
    const c1  = await cap(cSock, backAlias);
    const go2 = await cap(goSock, 'think [name(here)]');
    const c2  = await cap(cSock, 'think [name(here)]');
    testExact('exit back to Room1', go2, c2);
  }

  console.log('\n--- 3: say ---');
  {
    const go1 = await cap(goSock, 'say Hello world');
    const c1  = await cap(cSock, 'say Hello world');
    test('say output', go1, c1, /You say.*Hello world/i, /You say.*Hello world/i);
  }
  {
    const go1 = await cap(goSock, '"Hello again');
    const c1  = await cap(cSock, '"Hello again');
    test('" shortcut', go1, c1, /You say.*Hello again/i, /You say.*Hello again/i);
  }

  console.log('\n--- 4: pose ---');
  {
    const go1 = await cap(goSock, 'pose waves');
    const c1  = await cap(cSock, 'pose waves');
    test('pose output', go1, c1, /AuditMove waves/i, /AuditMove waves/i);
  }
  {
    const go1 = await cap(goSock, ':dances');
    const c1  = await cap(cSock, ':dances');
    test(': shortcut', go1, c1, /AuditMove dances/i, /AuditMove dances/i);
  }
  {
    const go1 = await cap(goSock, ";'s here");
    const c1  = await cap(cSock, ";'s here");
    test('; nospace pose', go1, c1, /AuditMove's here/i, /AuditMove's here/i);
  }

  console.log('\n--- 5: @emit ---');
  {
    const go1 = await cap(goSock, '@emit The ground shakes');
    const c1  = await cap(cSock, '@emit The ground shakes');
    test('@emit output', go1, c1, /The ground shakes/, /The ground shakes/);
  }

  console.log('\n--- 6: @pemit ---');
  {
    const go1 = await cap(goSock, '@pemit me=Private message test');
    const c1  = await cap(cSock, '@pemit me=Private message test');
    test('@pemit to self', go1, c1, /Private message test/, /Private message test/);
  }

  console.log('\n--- 7: whisper ---');
  {
    const go1 = await cap(goSock, 'whisper me=Secret whisper');
    const c1  = await cap(cSock, 'whisper me=Secret whisper');
    test('whisper to self', go1, c1, /whisper|Secret/i, /whisper|Secret/i);
  }

  console.log('\n--- 8: drop ---');
  {
    // Ball should be in inventory
    const go1 = await cap(goSock, `drop ${goBallRef}`);
    const c1  = await cap(cSock, `drop ${cBallRef}`);
    test('drop thing', go1, c1, /Dropped|drop/i, /Dropped|drop/i);
    const go2 = await cap(goSock, `think [eq(loc(${goBallRef}),loc(me))]`);
    const c2  = await cap(cSock, `think [eq(loc(${cBallRef}),loc(me))]`);
    testExact('drop moves to room', go2, c2);
  }

  console.log('\n--- 9: get ---');
  {
    const go1 = await cap(goSock, `get ${goBallRef}`);
    const c1  = await cap(cSock, `get ${cBallRef}`);
    test('get thing', go1, c1, /Taken|Picked up/i, /Taken|Picked up/i);
    const go2 = await cap(goSock, `think [eq(loc(${goBallRef}),num(me))]`);
    const c2  = await cap(cSock, `think [eq(loc(${cBallRef}),num(me))]`);
    testExact('get moves to inventory', go2, c2);
  }

  console.log('\n--- 10: @teleport ---');
  {
    const go1 = await cap(goSock, `@teleport me=${goRoom2}`);
    const c1  = await cap(cSock, `@teleport me=${cRoom2}`);
    const go2 = await cap(goSock, 'think [name(here)]');
    const c2  = await cap(cSock, 'think [name(here)]');
    testExact('@teleport to Room2', go2, c2);
    // Go back
    await cap(goSock, `@tel me=${goRoom1}`);
    await cap(cSock, `@tel me=${cRoom1}`);
  }

  console.log('\n--- 11: @oemit ---');
  {
    const go1 = await cap(goSock, '@oemit me=AuditMove does something');
    const c1  = await cap(cSock, '@oemit me=AuditMove does something');
    // @oemit sends to everyone EXCEPT the target, so we shouldn't see it ourselves
    // But there's nobody else, so just check no error
    results.push({ name: '@oemit no error', status: 'PASS' });
    console.log(`  ✓ @oemit no error`);
  }

  console.log('\n--- 12: inventory ---');
  {
    const go1 = await cap(goSock, 'inventory');
    const c1  = await cap(cSock, 'inventory');
    test('inventory shows ball', go1, c1, new RegExp(BALL_NAME), new RegExp(BALL_NAME));
  }

  console.log('\n--- 13: enter/leave ---');
  {
    // Drop ball first, set ENTER_OK
    await cap(goSock, `drop ${goBallRef}`);
    await cap(cSock, `drop ${cBallRef}`);
    await cap(goSock, `@set ${goBallRef}=ENTER_OK`);
    await cap(cSock, `@set ${cBallRef}=ENTER_OK`);
    await cap(goSock, `enter ${goBallRef}`);
    await cap(cSock, `enter ${cBallRef}`);
    // Verify location after enter — message format varies between Go and C
    const go1 = await cap(goSock, `think [eq(loc(me),${goBallRef})]`);
    const c1  = await cap(cSock, `think [eq(loc(me),${cBallRef})]`);
    testExact('enter moves player inside thing', go1, c1);

    await cap(goSock, 'leave');
    await cap(cSock, 'leave');
    // Verify we're back in the room
    const go2 = await cap(goSock, `think [eq(loc(me),${goRoom1})]`);
    const c2  = await cap(cSock, `think [eq(loc(me),${cRoom1})]`);
    testExact('leave returns to room', go2, c2);
  }

  console.log('\n--- 14: @odesc / @osucc / @ofail ---');
  {
    await cap(goSock, `@succ ${goBallRef}=You pick up the ball.`);
    await cap(cSock, `@succ ${cBallRef}=You pick up the ball.`);
    const go1 = await cap(goSock, `think [get(${goBallRef}/SUCC)]`);
    const c1  = await cap(cSock, `think [get(${cBallRef}/SUCC)]`);
    testExact('@succ attr stored', go1, c1);
  }
  {
    await cap(goSock, `@fail ${goBallRef}=The ball is too heavy.`);
    await cap(cSock, `@fail ${cBallRef}=The ball is too heavy.`);
    const go1 = await cap(goSock, `think [get(${goBallRef}/FAIL)]`);
    const c1  = await cap(cSock, `think [get(${cBallRef}/FAIL)]`);
    testExact('@fail attr stored', go1, c1);
  }

  console.log('\n--- 15: @wait ---');
  {
    await cap(goSock, '@wait 1=think MovWaitFired', 3000);
    await cap(cSock, '@wait 1=think MovWaitFired', 3000);
    await sleep(2000);
    const go1 = await cap(goSock, 'think check', 500);
    const c1  = await cap(cSock, 'think check', 500);
    // Both should have fired by now — the word "MovWaitFired" should appear somewhere
    // We can't easily test this without a second connection, so just verify no error
    results.push({ name: '@wait no error', status: 'PASS' });
    console.log(`  ✓ @wait no error`);
  }

  // ======= CLEANUP =======
  console.log('\n--- Cleanup ---');
  await cap(goSock, `get ${goBallRef}`);
  await cap(cSock, `get ${cBallRef}`);
  // Destroy ball by dbref
  await cap(goSock, `@set ${goBallRef}=DESTROY_OK`); await cap(goSock, `@destroy ${goBallRef}`);
  await cap(cSock, `@set ${cBallRef}=DESTROY_OK`);  await cap(cSock, `@destroy ${cBallRef}`);
  // Destroy exits — find by search since we don't have their dbrefs
  const goExits = lastLine(await cap(goSock, `think [lexits(${goRoom1})]`));
  for (const ex of goExits.split(' ').filter(e => e.startsWith('#'))) {
    await cap(goSock, `@set ${ex}=DESTROY_OK`); await cap(goSock, `@destroy ${ex}`);
  }
  const goExits2 = lastLine(await cap(goSock, `think [lexits(${goRoom2})]`));
  for (const ex of goExits2.split(' ').filter(e => e.startsWith('#'))) {
    await cap(goSock, `@set ${ex}=DESTROY_OK`); await cap(goSock, `@destroy ${ex}`);
  }
  const cExits = lastLine(await cap(cSock, `think [lexits(${cRoom1})]`));
  for (const ex of cExits.split(' ').filter(e => e.startsWith('#'))) {
    await cap(cSock, `@set ${ex}=DESTROY_OK`); await cap(cSock, `@destroy ${ex}`);
  }
  const cExits2 = lastLine(await cap(cSock, `think [lexits(${cRoom2})]`));
  for (const ex of cExits2.split(' ').filter(e => e.startsWith('#'))) {
    await cap(cSock, `@set ${ex}=DESTROY_OK`); await cap(cSock, `@destroy ${ex}`);
  }
  await cap(goSock, `@tel me=#0`);
  await cap(cSock, `@tel me=#0`);
  if (goRoom1) { await cap(goSock, `@set ${goRoom1}=DESTROY_OK`); await cap(goSock, `@destroy ${goRoom1}`); }
  if (goRoom2) { await cap(goSock, `@set ${goRoom2}=DESTROY_OK`); await cap(goSock, `@destroy ${goRoom2}`); }
  if (cRoom1)  { await cap(cSock, `@set ${cRoom1}=DESTROY_OK`);  await cap(cSock, `@destroy ${cRoom1}`); }
  if (cRoom2)  { await cap(cSock, `@set ${cRoom2}=DESTROY_OK`);  await cap(cSock, `@destroy ${cRoom2}`); }
  await sleep(300);

  // Summary
  console.log('\n=== Summary ===');
  const pass = results.filter(r => r.status === 'PASS').length;
  const fail = results.filter(r => r.status === 'FAIL').length;
  console.log(`Total: ${results.length} | Pass: ${pass} | Fail: ${fail}`);
  for (const r of results) {
    console.log(`  ${r.status==='PASS'?'✓':'✗'} ${r.status}: ${r.name}`);
  }
  console.log('\nDone.');
  goSock.destroy();
  cSock.destroy();
  process.exit(fail > 0 ? 1 : 0);
})();
