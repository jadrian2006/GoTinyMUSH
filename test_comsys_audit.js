#!/usr/bin/env node
// Comsys (channel) function audit: Go vs C
'use strict';

const net = require('net');

const HOST = process.env.MUSH_HOST || '192.168.100.12';
const GO_PORT = 6886;
const C_PORT = 9886;
const GO_LOGIN = 'connect Otter crystal';
const C_LOGIN = 'connect Otter crystal';

const tests = [
  // --- comlist ---
  ['gt(strlen(comlist()), 0)', 'comlist returns data'],

  // --- cwho ---
  // cwho returns connected listeners — subscriber state differs between DBs
  // Test that it returns valid output (not #-1 error) for an existing channel
  ['not(strmatch(cwho(Public),#-1*))', 'cwho Public does not error'],

  // --- cwhoall ---
  // cwhoall returns all subscribed players — subscriber lists differ between DBs
  ['gt(strlen(cwhoall(Public)), 0)', 'cwhoall Public has members'],

  // --- cominfo ---
  ['cominfo(Public)', 'cominfo Public'],

  // --- comheader ---
  // Header data differs between DBs (ANSI vs plain)
  ['gt(strlen(comheader(Public)), 0)', 'comheader Public non-empty'],

  // --- comalias ---
  ['comalias(me, Public)', 'comalias self'],

  // --- comdesc ---
  // Description content differs between DBs — test function works (no error)
  ['not(strmatch(comdesc(Public),#-1*))', 'comdesc Public does not error'],

  // --- comowner ---
  // Owner dbrefs differ between DBs
  ['gt(strlen(comowner(Public)), 0)', 'comowner Public exists'],

  // --- comtitle ---
  ['comtitle(me, Public)', 'comtitle self'],

  // --- cemit (side-effect, just test it accepts args) ---
  // skip cemit to avoid spamming channels
  ['cemit(Public, test)', 'cemit basic', 'skip'],
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
      skipped.push({ expr, desc, reason: 'Side-effect or unsafe' });
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

  console.log(`\n=== Comsys Function Audit ===`);
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
