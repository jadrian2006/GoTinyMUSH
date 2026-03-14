#!/usr/bin/env node
// Substitution audit: %subs, bracket eval, escape sequences, register scoping
// Go (192.168.100.12:6886) vs C (192.168.100.12:9886)
// Uses AuditSubs/auditpass account (WIZARD, in own room)
'use strict';

const net = require('net');
const HOST = process.env.MUSH_HOST || '192.168.100.12';
const GO_PORT = 6886;
const C_PORT  = 9886;
const GO_LOGIN = process.env.GO_LOGIN || 'connect AuditSubs auditpass';
const C_LOGIN  = process.env.C_LOGIN || 'connect AuditSubs auditpass';

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
    const marker = 'XSUB' + Date.now() + Math.random().toString(36).slice(2,6) + 'X';
    const onData = d => { buf += d; if (buf.includes(marker)) { sock.off('data', onData); resolve(buf); } };
    sock.on('data', onData);
    sock.write(cmd + '\n');
    sock.write(`think ${marker}\n`);
    setTimeout(() => { sock.off('data', onData); resolve(buf); }, ms);
  });
}

function lastLine(buf) {
  return buf.replace(/XSUB\S+X/g,'').trim().split('\n').filter(l=>l.trim()&&!l.includes('XSUB')).pop()?.trim()||'';
}

async function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

const results = [];
function testExact(name, go1, c1) {
  const g = go1.replace(/XSUB\S+X/g,'').trim().split('\n').filter(l=>l.trim()).pop()?.trim()||'';
  const c = c1.replace(/XSUB\S+X/g,'').trim().split('\n').filter(l=>l.trim()).pop()?.trim()||'';
  const pass = g === c;
  results.push({ name, status: pass ? 'PASS' : 'FAIL' });
  console.log(`  ${pass?'✓':'✗'} ${name}`);
  if (!pass) { console.log(`    Go: ${g}`); console.log(`     C: ${c}`); }
}

function test(name, goOut, cOut, goP, cP) {
  const goMatch = goP instanceof RegExp ? goP.test(goOut) : goOut.includes(goP);
  const cMatch = cP === 'SKIP' ? true : cP instanceof RegExp ? cP.test(cOut) : cOut.includes(cP);
  const status = (goMatch && cMatch) ? 'PASS' : 'FAIL';
  results.push({ name, status });
  console.log(`  ${status==='PASS'?'✓':'✗'} ${name}`);
  if (status === 'FAIL') {
    if (!goMatch) console.log(`    Go FAIL: ${goOut.replace(/XSUB\S+X/g,'').trim().split('\n').slice(0,3).join(' | ')}`);
    if (!cMatch && cP !== 'SKIP') console.log(`    C  FAIL: ${cOut.replace(/XSUB\S+X/g,'').trim().split('\n').slice(0,3).join(' | ')}`);
  }
}

(async () => {
  console.log(`=== Substitution Audit: Go (:${GO_PORT}) vs C (:${C_PORT}) on ${HOST} ===\n`);

  const goSock = await connect(GO_PORT);
  const cSock  = await connect(C_PORT);
  await sleep(500);
  goSock.write(GO_LOGIN + '\n');
  cSock.write(C_LOGIN + '\n');
  await sleep(1000);
  await cap(goSock, 'think go-ready', 500);
  await cap(cSock, 'think c-ready', 500);

  // Set sex for pronoun tests
  await cap(goSock, '@sex me=male');
  await cap(cSock, '@sex me=male');
  await sleep(300);

  // Create test object for u() scoping
  await cap(goSock, '@create SubWidget');
  await cap(cSock, '@create SubWidget');
  await cap(goSock, '&UFUN SubWidget=got-%0-%1');
  await cap(cSock, '&UFUN SubWidget=got-%0-%1');
  await sleep(300);

  // ======= 1: Basic %substitutions =======
  console.log('--- 1: % substitutions ---');
  {
    // %n = enactor name
    const go1 = await cap(goSock, 'think %n');
    const c1  = await cap(cSock, 'think %n');
    testExact('%n = name', go1, c1);
  }
  {
    // %# = enactor dbref
    const go1 = await cap(goSock, 'think [type(%#)]');
    const c1  = await cap(cSock, 'think [type(%#)]');
    testExact('%# type is PLAYER', go1, c1);
  }
  {
    // %l = location
    const go1 = await cap(goSock, 'think [type(%l)]');
    const c1  = await cap(cSock, 'think [type(%l)]');
    testExact('%l type is ROOM', go1, c1);
  }
  {
    // %b = space
    const go1 = await cap(goSock, 'think X%bY');
    const c1  = await cap(cSock, 'think X%bY');
    testExact('%b = space', go1, c1);
  }
  {
    // %r = newline (check by strlen — %r adds a character)
    const go1 = await cap(goSock, 'think [strlen(A%rB)]');
    const c1  = await cap(cSock, 'think [strlen(A%rB)]');
    testExact('%r strlen = 3', go1, c1);
  }
  {
    // %t = tab
    const go1 = await cap(goSock, 'think [strlen(A%tB)]');
    const c1  = await cap(cSock, 'think [strlen(A%tB)]');
    testExact('%t strlen = 3', go1, c1);
  }
  {
    // %% = literal %
    const go1 = await cap(goSock, 'think %%');
    const c1  = await cap(cSock, 'think %%');
    testExact('%% = literal %', go1, c1);
  }

  // ======= 2: Pronoun substitutions =======
  console.log('\n--- 2: Pronouns ---');
  {
    const go1 = await cap(goSock, 'think %s');
    const c1  = await cap(cSock, 'think %s');
    testExact('%s subjective', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think %o');
    const c1  = await cap(cSock, 'think %o');
    testExact('%o objective', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think %p');
    const c1  = await cap(cSock, 'think %p');
    testExact('%p possessive', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think %a');
    const c1  = await cap(cSock, 'think %a');
    testExact('%a absolute possessive', go1, c1);
  }

  // ======= 3: Bracket evaluation =======
  console.log('\n--- 3: Bracket evaluation ---');
  {
    const go1 = await cap(goSock, 'think [add(1,2)]');
    const c1  = await cap(cSock, 'think [add(1,2)]');
    testExact('[add(1,2)]', go1, c1);
  }
  {
    // Nested brackets
    const go1 = await cap(goSock, 'think [add(mul(2,3),4)]');
    const c1  = await cap(cSock, 'think [add(mul(2,3),4)]');
    testExact('nested [add(mul(2,3),4)]', go1, c1);
  }
  {
    // Multiple brackets in sequence
    const go1 = await cap(goSock, 'think [add(1,1)]-[mul(2,2)]');
    const c1  = await cap(cSock, 'think [add(1,1)]-[mul(2,2)]');
    testExact('multi bracket sequence', go1, c1);
  }
  {
    // Escaped bracket
    const go1 = await cap(goSock, 'think \\[not evaluated\\]');
    const c1  = await cap(cSock, 'think \\[not evaluated\\]');
    testExact('\\[ escaped bracket', go1, c1);
  }

  // ======= 4: Register scoping =======
  console.log('\n--- 4: Register scoping ---');
  {
    // Basic setq/r
    const go1 = await cap(goSock, 'think [setq(0,hello)][r(0)]');
    const c1  = await cap(cSock, 'think [setq(0,hello)][r(0)]');
    testExact('setq/r basic', go1, c1);
  }
  {
    // Named registers
    const go1 = await cap(goSock, 'think [setq(name,test)][r(name)]');
    const c1  = await cap(cSock, 'think [setq(name,test)][r(name)]');
    testExact('named register', go1, c1);
  }
  {
    // setr returns value
    const go1 = await cap(goSock, 'think [setr(0,value)]');
    const c1  = await cap(cSock, 'think [setr(0,value)]');
    testExact('setr returns value', go1, c1);
  }
  {
    // Multiple registers
    const go1 = await cap(goSock, 'think [setq(0,A)][setq(1,B)][r(0)][r(1)]');
    const c1  = await cap(cSock, 'think [setq(0,A)][setq(1,B)][r(0)][r(1)]');
    testExact('multiple registers', go1, c1);
  }
  {
    // localize()
    const go1 = await cap(goSock, 'think [setq(0,outer)][localize(setq(0,inner)[r(0)])][r(0)]');
    const c1  = await cap(cSock, 'think [setq(0,outer)][localize(setq(0,inner)[r(0)])][r(0)]');
    testExact('localize() scoping', go1, c1);
  }

  // ======= 5: u() function call substitutions =======
  console.log('\n--- 5: u() substitutions ---');
  {
    // %0, %1 in u()
    const go1 = await cap(goSock, 'think [u(SubWidget/UFUN,hello,world)]');
    const c1  = await cap(cSock, 'think [u(SubWidget/UFUN,hello,world)]');
    testExact('u() with %0 %1', go1, c1);
  }
  {
    // %# inside u()
    await cap(goSock, '&WHOFN SubWidget=type(%#)');
    await cap(cSock, '&WHOFN SubWidget=type(%#)');
    const go1 = await cap(goSock, 'think [u(SubWidget/WHOFN)]');
    const c1  = await cap(cSock, 'think [u(SubWidget/WHOFN)]');
    testExact('u() sees %# = PLAYER', go1, c1);
  }

  // ======= 6: Escape sequences =======
  console.log('\n--- 6: Escape sequences ---');
  {
    // Backslash escapes
    const go1 = await cap(goSock, 'think \\\\');
    const c1  = await cap(cSock, 'think \\\\');
    testExact('\\\\ = literal backslash', go1, c1);
  }
  {
    // Curly braces
    const go1 = await cap(goSock, 'think {literal braces}');
    const c1  = await cap(cSock, 'think {literal braces}');
    testExact('{} passes through', go1, c1);
  }
  {
    // Comma in function args — should be arg separator
    const go1 = await cap(goSock, 'think [add(1,2,3)]');
    const c1  = await cap(cSock, 'think [add(1,2,3)]');
    testExact('comma as arg separator', go1, c1);
  }

  // ======= 7: iter() substitutions =======
  console.log('\n--- 7: iter() substitutions ---');
  {
    const go1 = await cap(goSock, 'think [iter(a b c,##)]');
    const c1  = await cap(cSock, 'think [iter(a b c,##)]');
    testExact('iter ## substitution', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think [iter(a b c,#@)]');
    const c1  = await cap(cSock, 'think [iter(a b c,#@)]');
    testExact('iter #@ counter', go1, c1);
  }
  {
    // Nested iter
    const go1 = await cap(goSock, 'think [iter(1 2,iter(a b,itext(1)-##))]');
    const c1  = await cap(cSock, 'think [iter(1 2,iter(a b,itext(1)-##))]');
    testExact('nested iter itext(1)', go1, c1);
  }

  // ======= 8: switch() function =======
  console.log('\n--- 8: switch() function ---');
  {
    const go1 = await cap(goSock, 'think [switch(abc,abc,yes,no)]');
    const c1  = await cap(cSock, 'think [switch(abc,abc,yes,no)]');
    testExact('switch() match', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think [switch(abc,def,yes,no)]');
    const c1  = await cap(cSock, 'think [switch(abc,def,yes,no)]');
    testExact('switch() default', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think [switch(abc,a*,wild,no)]');
    const c1  = await cap(cSock, 'think [switch(abc,a*,wild,no)]');
    testExact('switch() wildcard', go1, c1);
  }

  // ======= 9: ifelse / if =======
  console.log('\n--- 9: ifelse / if ---');
  {
    const go1 = await cap(goSock, 'think [ifelse(1,true,false)]');
    const c1  = await cap(cSock, 'think [ifelse(1,true,false)]');
    testExact('ifelse true', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think [ifelse(0,true,false)]');
    const c1  = await cap(cSock, 'think [ifelse(0,true,false)]');
    testExact('ifelse false', go1, c1);
  }
  {
    // Nested ifelse — verify independent of if()
    const go1 = await cap(goSock, 'think [ifelse(1,ifelse(0,inner-true,inner-false),outer-false)]');
    const c1  = await cap(cSock, 'think [ifelse(1,ifelse(0,inner-true,inner-false),outer-false)]');
    testExact('ifelse nested', go1, c1);
  }
  {
    // if() — C 3.1 doesn't have if(), only ifelse()
    const go1 = await cap(goSock, 'think [if(1,true)]');
    const c1  = await cap(cSock, 'think [if(1,true)]');
    // Check Go returns 'true' regardless of C
    const g = go1.replace(/XSUB\S+X/g,'').trim().split('\n').filter(l=>l.trim()).pop()?.trim()||'';
    const goOk = g === 'true';
    results.push({ name: 'if() true (C 3.1 gap)', status: goOk ? 'CBUG' : 'FAIL' });
    console.log(`  ${goOk ? '⚠' : '✗'} if() true (C 3.1 gap)`);
    if (!goOk) console.log(`    Go: ${g} (expected true)`);
  }
  {
    const go1 = await cap(goSock, 'think [if(0,true)]');
    const c1  = await cap(cSock, 'think [if(0,true)]');
    const g = go1.replace(/XSUB\S+X/g,'').trim().split('\n').filter(l=>l.trim()).pop()?.trim()||'';
    const goOk = g === '';
    results.push({ name: 'if() false empty (C 3.1 gap)', status: goOk ? 'CBUG' : 'FAIL' });
    console.log(`  ${goOk ? '⚠' : '✗'} if() false empty (C 3.1 gap)`);
    if (!goOk) console.log(`    Go: ${g} (expected empty)`);
  }

  // ======= 10: v() =======
  console.log('\n--- 10: v() built-in attrs ---');
  {
    await cap(goSock, '@desc me=AuditSubs test desc');
    await cap(cSock, '@desc me=AuditSubs test desc');
    const go1 = await cap(goSock, 'think [v(desc)]');
    const c1  = await cap(cSock, 'think [v(desc)]');
    testExact('v(desc)', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think [v(n)]');
    const c1  = await cap(cSock, 'think [v(n)]');
    testExact('v(n) = name', go1, c1);
  }

  // ======= 11: objeval scoping =======
  console.log('\n--- 11: objeval ---');
  {
    const go1 = await cap(goSock, 'think [objeval(SubWidget,name(me))]');
    const c1  = await cap(cSock, 'think [objeval(SubWidget,name(me))]');
    testExact('objeval(obj,name(me))', go1, c1);
  }

  // ======= 12: secure/escape =======
  console.log('\n--- 12: secure/escape ---');
  {
    const go1 = await cap(goSock, 'think [escape(hello [world])]');
    const c1  = await cap(cSock, 'think [escape(hello [world])]');
    testExact('escape() brackets', go1, c1);
  }
  {
    const go1 = await cap(goSock, 'think [strlen(secure(test%r%n))]');
    const c1  = await cap(cSock, 'think [strlen(secure(test%r%n))]');
    testExact('secure() length', go1, c1);
  }

  // ======= CLEANUP =======
  console.log('\n--- Cleanup ---');
  await cap(goSock, '@set SubWidget=DESTROY_OK'); await cap(goSock, '@destroy SubWidget');
  await cap(cSock, '@set SubWidget=DESTROY_OK');  await cap(cSock, '@destroy SubWidget');
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
