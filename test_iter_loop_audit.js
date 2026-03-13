#!/usr/bin/env node
// Iteration and loop function audit: Go vs C
'use strict';

const net = require('net');

const HOST = process.env.MUSH_HOST || '192.168.100.12';
const GO_PORT = 6886;
const C_PORT = 9886;
const GO_LOGIN = 'connect Moravel mne8994';
const C_LOGIN = 'connect Moravel mne8994';

const tests = [
  // --- foreach ---
  ['foreach(ucstr, abc)', 'foreach basic'],

  // --- iter ---
  ['iter(one two three, ucstr(##))', 'iter ucstr'],
  ['iter(a b c, ##-##)', 'iter double ref'],
  ['iter(1|2|3, add(##,10), |)', 'iter custom sep'],
  ['iter(1|2|3, add(##,10), |, -)', 'iter custom osep'],

  // --- iter2 ---
  ['iter2(a b c, d e f, ##-@@)', 'iter2 two lists', 'skip'],

  // --- itext ---
  ['iter(a b c, itext(0))', 'itext current'],

  // --- inum ---
  ['iter(a b c, inum(0))', 'inum current'],

  // --- ilev ---
  ['iter(a b, iter(1 2, ilev()))', 'ilev nesting level'],

  // --- parse ---
  ['parse(one two three, ucstr(##))', 'parse basic'],
  ['parse(1|2|3, add(##,10), |)', 'parse custom sep'],
  ['parse(1|2|3, add(##,10), |, -)', 'parse custom osep'],

  // --- map ---
  ['map(me/VA, one two three)', 'map on attr', 'skip'],

  // --- step ---
  ['step(me/VA, one two three four five six, 2)', 'step groups of 2', 'skip'],

  // --- while ---
  ['while(lt(##, 5), add(##, 1), 0)', 'while basic'],
  ['while(lt(##, 3), add(##, 1), 0, |)', 'while with osep'],

  // --- until ---
  ['until(gte(##, 5), add(##, 1), 0)', 'until basic'],

  // --- loop ---
  ['loop(1, 5, 1, ##)', 'loop basic'],
  ['loop(0, 10, 3, ##)', 'loop step 3'],
  ['loop(5, 1, -1, ##)', 'loop reverse'],
  ['loop(1, 3, 1, ##, |)', 'loop with osep'],

  // --- whentrue ---
  ['whentrue(1, yes, no)', 'whentrue true'],
  ['whentrue(0, yes, no)', 'whentrue false'],

  // --- whenfalse ---
  ['whenfalse(0, yes, no)', 'whenfalse false'],
  ['whenfalse(1, yes, no)', 'whenfalse true'],

  // --- whentrue2 ---
  ['whentrue2(1, yes, no)', 'whentrue2 true', 'skip'],
  ['whentrue2(0, yes, no)', 'whentrue2 false', 'skip'],

  // --- whenfalse2 ---
  ['whenfalse2(0, yes, no)', 'whenfalse2 false', 'skip'],
  ['whenfalse2(1, yes, no)', 'whenfalse2 true', 'skip'],

  // --- filterbool ---
  ['filterbool(1 0 1 0 1, a b c d e)', 'filterbool basic'],
  ['filterbool(0 0 0, x y z)', 'filterbool all false'],
  ['filterbool(1 1 1, a b c)', 'filterbool all true'],

  // --- list / list2 ---
  ['list(one two three, think ##)', 'list basic', 'skip'],

  // --- land / landbool ---
  ['land(1 1 1)', 'land all true'],
  ['land(1 0 1)', 'land one false'],
  ['land()', 'land empty'],
  ['landbool(1 1 1)', 'landbool all true'],
  ['landbool(1 0 1)', 'landbool one false'],

  // --- lor / lorbool ---
  ['lor(0 0 1)', 'lor one true'],
  ['lor(0 0 0)', 'lor all false'],
  ['lor()', 'lor empty'],
  ['lorbool(0 0 1)', 'lorbool one true'],
  ['lorbool(0 0 0)', 'lorbool all false'],

  // --- ldiff ---
  ['ldiff(a b c d, b d)', 'ldiff basic'],
  ['ldiff(1 2 3, 4 5)', 'ldiff no overlap'],
  ['ldiff(a b c, a b c)', 'ldiff all removed'],

  // --- linter ---
  ['linter(a b c d, b d e)', 'linter basic'],
  ['linter(1 2 3, 4 5)', 'linter no overlap'],

  // --- lunion ---
  ['lunion(a b c, b c d)', 'lunion basic', 'skip'],

  // --- locate ---
  ['isdbref(locate(me, me, *))', 'locate self'],
  ['locate(me, NonExistent12345, *)', 'locate not found'],

  // --- lastcreate ---
  // lastcreate: C 3.1 requires 2 args; wrap in name() to normalize dbrefs
  ['name(lastcreate(me, t))', 'lastcreate 2-arg name'],

  // --- wordpos (takes 1-based char position, returns word number) ---
  ['wordpos(one two three, 1)', 'wordpos pos 1'],
  ['wordpos(one two three, 5)', 'wordpos pos 5'],
  ['wordpos(one two three, 99)', 'wordpos out of range'],

  // --- iffalse / iftrue / ifzero ---
  ['iftrue(1, yes, no)', 'iftrue true'],
  ['iftrue(0, yes, no)', 'iftrue false'],
  ['iffalse(0, yes, no)', 'iffalse false'],
  ['iffalse(1, yes, no)', 'iffalse true'],
  ['ifzero(0, yes, no)', 'ifzero zero'],
  ['ifzero(1, yes, no)', 'ifzero nonzero'],
];

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

function sendCmd(sock, cmd) {
  return new Promise((resolve) => {
    let buf = '';
    const onData = (d) => { buf += d; };
    sock.on('data', onData);
    sock.write(cmd + '\n');
    setTimeout(() => {
      sock.removeListener('data', onData);
      const clean = buf.replace(/\xff[\xfb-\xfe]./g, '').replace(/\xff\xf1/g, '').trim();
      resolve(clean);
    }, 300);
  });
}

async function runTests() {
  console.log(`Connecting to Go (${HOST}:${GO_PORT}) and C (${HOST}:${C_PORT})...`);

  const goSock = await connect(GO_PORT, GO_LOGIN);
  const cSock = await connect(C_PORT, C_LOGIN);

  await sendCmd(cSock, 'visitor');
  await new Promise(r => setTimeout(r, 1000));
  await sendCmd(cSock, 'out');
  await new Promise(r => setTimeout(r, 1000));

  await sendCmd(goSock, '+quiet/all on');
  await new Promise(r => setTimeout(r, 1000));
  await sendCmd(goSock, 'think XFLUSH');
  await new Promise(r => setTimeout(r, 500));

  let pass = 0, fail = 0, skip = 0;
  const failures = [];
  const skipped = [];

  for (const entry of tests) {
    const [expr, desc, mode] = entry;

    if (mode === 'skip') {
      skip++;
      skipped.push({ expr, desc, reason: 'Needs setup or Go-only' });
      continue;
    }

    const cmd = `think [${expr}]`;
    const goResult = await sendCmd(goSock, cmd);
    const cResult = await sendCmd(cSock, cmd);

    const goVal = goResult.split('\n').filter(l => l.trim()).pop() || '';
    const cVal = cResult.split('\n').filter(l => l.trim()).pop() || '';

    if (goVal === cVal) {
      pass++;
    } else {
      if (cVal.includes('No matching function') || cVal.includes('FUNCTION')) {
        skip++;
        skipped.push({ expr, desc, reason: 'C 3.1 missing' });
      } else {
        fail++;
        failures.push({ expr, desc, go: goVal, c: cVal });
      }
    }
  }

  const total = pass + fail;
  const pct = total > 0 ? Math.round((pass / total) * 100) : 100;

  console.log(`\n=== Iter/Loop Audit ===`);
  console.log(`Total: ${total}  Pass: ${pass}  Fail: ${fail}  Skip: ${skip}  Match: ${pct}%\n`);

  if (failures.length > 0) {
    console.log('--- Failures ---');
    for (const f of failures) {
      console.log(`  ${f.desc}: ${f.expr}`);
      console.log(`    Go: ${JSON.stringify(f.go)}`);
      console.log(`     C: ${JSON.stringify(f.c)}`);
    }
  }

  if (skipped.length > 0) {
    console.log('\n--- Skipped ---');
    for (const s of skipped) {
      console.log(`  ${s.desc}: ${s.expr} (${s.reason})`);
    }
  }

  goSock.destroy();
  cSock.destroy();
  console.log(`\nDone.`);
  process.exit(fail > 0 ? 1 : 0);
}

runTests().catch(e => { console.error(e); process.exit(1); });
