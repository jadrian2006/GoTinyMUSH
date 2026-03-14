#!/usr/bin/env node
// Object function comparison: Go vs C on Atlas (192.168.100.12)
// Creates identical test objects on both servers, compares output.
// Dbrefs differ between DBs — comparisons normalize dbrefs to names.
'use strict';

const net = require('net');

const HOST = '192.168.100.12';
const GO_PORT = 6886;
const C_PORT = 9886;
const GO_LOGIN = 'connect Moravel mne8994';
const C_LOGIN = 'connect Moravel mne8994';

// No static setup cmds — we use create() function to capture dbrefs

// After setup, we query for dbrefs and build dynamic tests
// This 2-phase approach handles the different dbrefs

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
    }, 400);
  });
}

function lastLine(s) {
  return (s.split('\n').filter(l => l.trim()).pop() || '').trim();
}

// Normalize dbrefs in a string to [name] format
// Replaces #NNN with the name lookup result
function normalizeDbrefs(s, dbrefMap) {
  return s.replace(/#(\d+)/g, (match, num) => {
    const name = dbrefMap['#' + num];
    return name ? `[${name}]` : match;
  });
}

async function runTests() {
  console.log(`Connecting to Go (${HOST}:${GO_PORT}) and C (${HOST}:${C_PORT})...`);

  const goSock = await connect(GO_PORT, GO_LOGIN);
  const cSock = await connect(C_PORT, C_LOGIN);

  // Quiet mode on Go
  await sendCmd(goSock, '+quiet/all on');
  await new Promise(r => setTimeout(r, 1000));
  await sendCmd(goSock, 'think XFLUSH');
  await new Promise(r => setTimeout(r, 500));

  // Setup: dig dedicated test room, teleport in, create objects there
  console.log('Setting up test environment...');

  // Save original locations to restore later
  const goOrigLoc = lastLine(await sendCmd(goSock, 'think [loc(me)]'));
  const cOrigLoc = lastLine(await sendCmd(cSock, 'think [loc(me)]'));

  // Set sex/desc on Moravel (may be needed to satisfy teleport locks)
  await sendCmd(goSock, '@sex me=male');
  await sendCmd(goSock, '@desc me=building wizard');
  await sendCmd(cSock, '@sex me=male');
  await sendCmd(cSock, '@desc me=building wizard');
  // Fix home if pointing to garbage — set to #0
  const goHomeType = lastLine(await sendCmd(goSock, 'think [type(home(me))]'));
  if (goHomeType !== 'ROOM') {
    console.log(`  Go home is ${goHomeType}, fixing to #0...`);
    await sendCmd(goSock, '@link me=#0');
  }
  await new Promise(r => setTimeout(r, 300));

  // C's Toaster Oven is a chargen room — need to type 'visitor' then 'out' to leave
  await sendCmd(cSock, 'visitor');
  await new Promise(r => setTimeout(r, 500));
  await sendCmd(cSock, 'out');
  await new Promise(r => setTimeout(r, 500));

  // @dig test room — parse room number from output
  const goDigOut = await sendCmd(goSock, '@dig AuditTestRoom');
  const cDigOut = await sendCmd(cSock, '@dig AuditTestRoom');
  await new Promise(r => setTimeout(r, 500));

  function parseRoomNum(output) {
    const m = output.match(/room number (\d+)/);
    return m ? '#' + m[1] : null;
  }
  const goChamberRef = parseRoomNum(goDigOut);
  const cChamberRef = parseRoomNum(cDigOut);
  console.log(`  Go room: ${goChamberRef}, C room: ${cChamberRef}`);

  if (!goChamberRef || !cChamberRef) {
    console.error('ERROR: Failed to parse room dbref from @dig output');
    console.error(`  Go output: ${goDigOut}`);
    console.error(`   C output: ${cDigOut}`);
    goSock.write('QUIT\n');
    cSock.write('QUIT\n');
    setTimeout(() => process.exit(1), 500);
    return;
  }

  // Teleport into test room
  await sendCmd(goSock, `@tel me=${goChamberRef}`);
  await sendCmd(cSock, `@tel me=${cChamberRef}`);
  await new Promise(r => setTimeout(r, 500));

  // Verify teleport
  const goVerifyLoc = lastLine(await sendCmd(goSock, 'think [loc(me)]'));
  const cVerifyLoc = lastLine(await sendCmd(cSock, 'think [loc(me)]'));
  console.log(`  Go loc: ${goVerifyLoc} (want ${goChamberRef})`);
  console.log(`   C loc: ${cVerifyLoc} (want ${cChamberRef})`);

  // Use unique names to avoid #-2 AMBIGUOUS from leftover objects
  const uid = Date.now().toString(36);
  const WIDGET_NAME = `AuditW${uid}`;
  const GADGET_NAME = `AuditG${uid}`;

  // Create objects — they start in player inventory
  const goWidgetRef = lastLine(await sendCmd(goSock, `think [create(${WIDGET_NAME},10)]`));
  const cWidgetRef = lastLine(await sendCmd(cSock, `think [create(${WIDGET_NAME},10)]`));
  const goGadgetRef = lastLine(await sendCmd(goSock, `think [create(${GADGET_NAME},10)]`));
  const cGadgetRef = lastLine(await sendCmd(cSock, `think [create(${GADGET_NAME},10)]`));

  // Set attrs and flags
  await sendCmd(goSock, `@set ${goWidgetRef}=VISUAL`);
  await sendCmd(cSock, `@set ${cWidgetRef}=VISUAL`);
  await sendCmd(goSock, `&TESTATTR ${goWidgetRef}=hello world`);
  await sendCmd(cSock, `&TESTATTR ${cWidgetRef}=hello world`);
  await sendCmd(goSock, `&NUMATTR ${goWidgetRef}=42`);
  await sendCmd(cSock, `&NUMATTR ${cWidgetRef}=42`);
  await sendCmd(goSock, `&UFUN ${goWidgetRef}=[add(%0,%1)]`);
  await sendCmd(cSock, `&UFUN ${cWidgetRef}=[add(%0,%1)]`);
  await sendCmd(goSock, `@set ${goGadgetRef}=DARK`);
  await sendCmd(cSock, `@set ${cGadgetRef}=DARK`);

  // Get player info
  const goMeRef = lastLine(await sendCmd(goSock, 'think [num(me)]'));
  const goLocRef = lastLine(await sendCmd(goSock, 'think [num(loc(me))]'));
  const goHomeRef = lastLine(await sendCmd(goSock, 'think [num(home(me))]'));
  const cMeRef = lastLine(await sendCmd(cSock, 'think [num(me)]'));
  const cLocRef = lastLine(await sendCmd(cSock, 'think [num(loc(me))]'));
  const cHomeRef = lastLine(await sendCmd(cSock, 'think [num(home(me))]'));

  console.log(`  Go: Widget=${goWidgetRef} Gadget=${goGadgetRef} Chamber=${goChamberRef} Me=${goMeRef}`);
  console.log(`   C: Widget=${cWidgetRef} Gadget=${cGadgetRef} Chamber=${cChamberRef} Me=${cMeRef}`);

  // Flush C output buffer (stale output from prior runs)
  await sendCmd(cSock, 'think XFLUSH_SETUP');
  await new Promise(r => setTimeout(r, 500));

  // Drop widget into room (from inventory)
  await sendCmd(goSock, `drop ${goWidgetRef}`);
  await sendCmd(cSock, `drop ${cWidgetRef}`);
  await new Promise(r => setTimeout(r, 300));
  // Set parent
  await sendCmd(goSock, `@parent ${goGadgetRef}=${goWidgetRef}`);
  await sendCmd(cSock, `@parent ${cGadgetRef}=${cWidgetRef}`);
  await new Promise(r => setTimeout(r, 500));
  // Flush again after setup
  await sendCmd(goSock, 'think XFLUSH2');
  await sendCmd(cSock, 'think XFLUSH2');
  await new Promise(r => setTimeout(r, 300));

  // Verify widget location
  const goWidgetLoc = lastLine(await sendCmd(goSock, `think [loc(${goWidgetRef})]`));
  const cWidgetLoc = lastLine(await sendCmd(cSock, `think [loc(${cWidgetRef})]`));
  console.log(`  Widget loc — Go: ${goWidgetLoc} C: ${cWidgetLoc}`);

  // Build dbref→name maps for normalization
  // We query names for all known dbrefs
  const goMap = {};
  const cMap = {};
  for (const [ref, label] of [[goWidgetRef,'Test Widget'],[goGadgetRef,'Test Gadget'],[goChamberRef,'Test Chamber'],[goMeRef,'Moravel'],[goLocRef,'MyLoc'],[goHomeRef,'MyHome']]) {
    goMap[ref] = label;
  }
  for (const [ref, label] of [[cWidgetRef,'Test Widget'],[cGadgetRef,'Test Gadget'],[cChamberRef,'Test Chamber'],[cMeRef,'Moravel'],[cLocRef,'MyLoc'],[cHomeRef,'MyHome']]) {
    cMap[ref] = label;
  }

  // Also get room #0 name for both
  const goRoom0Name = lastLine(await sendCmd(goSock, 'think [name(#0)]'));
  const cRoom0Name = lastLine(await sendCmd(cSock, 'think [name(#0)]'));
  goMap['#0'] = goRoom0Name || 'Room0';
  cMap['#0'] = cRoom0Name || 'Room0';
  // #1 (God)
  const goGodName = lastLine(await sendCmd(goSock, 'think [name(#1)]'));
  const cGodName = lastLine(await sendCmd(cSock, 'think [name(#1)]'));
  goMap['#1'] = goGodName || 'God';
  cMap['#1'] = cGodName || 'God';
  // #-1
  goMap['#-1'] = 'NOTHING';
  cMap['#-1'] = 'NOTHING';

  // Tests: [goExpr, cExpr, description, mode]
  // mode: 'exact' (default) = compare strings directly
  //       'dbref' = normalize dbrefs before comparison
  //       'bool' = compare truthiness (both truthy or both falsy)
  //       'skip' = known diff, log but don't fail
  const W = { go: goWidgetRef, c: cWidgetRef };
  const G = { go: goGadgetRef, c: cGadgetRef };
  const CH = { go: goChamberRef, c: cChamberRef };
  const ME = { go: goMeRef, c: cMeRef };

  // Helper to build test pairs with different dbrefs
  function t(goExpr, cExpr, desc, mode) {
    return [goExpr, cExpr, desc, mode || 'exact'];
  }
  function ts(expr, desc, mode) {
    // Same expression on both — no dbref substitution needed
    return [expr, expr, desc, mode || 'exact'];
  }
  function tw(template, desc, mode) {
    // Template with %W=widget, %G=gadget, %C=chamber, %M=me substitution
    const goExpr = template.replace(/%W/g, W.go).replace(/%G/g, G.go).replace(/%C/g, CH.go).replace(/%M/g, ME.go);
    const cExpr = template.replace(/%W/g, W.c).replace(/%G/g, G.c).replace(/%C/g, CH.c).replace(/%M/g, ME.c);
    return [goExpr, cExpr, desc, mode || 'exact'];
  }

  const tests = [
    // --- name ---
    tw('name(%W)', 'name of thing', 'exact'),
    tw('name(%C)', 'name of test room', 'exact'),
    tw('name(%M)', 'name of player', 'exact'),
    // name(#0) differs between DBs — skip
    ts('t(strlen(name(#0)))', 'name of #0 non-empty', 'exact'),

    // --- fullname --- (contains dbref — compare name portion only)
    t(`strmatch(fullname(${W.go}),${WIDGET_NAME}*)`, `strmatch(fullname(${W.c}),${WIDGET_NAME}*)`, 'fullname thing starts with name', 'exact'),
    tw('strmatch(fullname(%M),Moravel*)', 'fullname player starts with name', 'exact'),

    // --- type ---
    tw('type(%W)', 'type of thing', 'exact'),
    tw('type(%C)', 'type of room', 'exact'),
    tw('type(%M)', 'type of player', 'exact'),

    // --- hastype ---
    tw('hastype(%W,THING)', 'hastype thing=THING', 'exact'),
    tw('hastype(%W,ROOM)', 'hastype thing=ROOM', 'exact'),
    tw('hastype(%C,ROOM)', 'hastype room=ROOM', 'exact'),
    tw('hastype(%M,PLAYER)', 'hastype player=PLAYER', 'exact'),

    // --- flags ---
    tw('flags(%W)', 'flags of visual thing', 'exact'),
    tw('flags(%G)', 'flags of dark thing', 'exact'),

    // --- hasflag ---
    tw('hasflag(%W,VISUAL)', 'hasflag VISUAL', 'exact'),
    tw('hasflag(%W,DARK)', 'hasflag not DARK', 'exact'),
    tw('hasflag(%G,DARK)', 'hasflag DARK', 'exact'),
    tw('hasflag(%M,WIZARD)', 'hasflag WIZARD', 'exact'),

    // --- andflags ---
    tw('andflags(%G,D)', 'andflags D', 'exact'),
    tw('andflags(%G,DV)', 'andflags DV (not both)', 'exact'),

    // --- orflags ---
    tw('orflags(%G,DV)', 'orflags DV', 'exact'),
    tw('orflags(%G,XY)', 'orflags neither', 'exact'),

    // --- loc ---
    tw('name(loc(%W))', 'loc of thing name', 'exact'),
    tw('name(loc(%M))', 'loc of player name', 'exact'),

    // --- owner ---
    tw('name(owner(%W))', 'owner of thing name', 'exact'),
    tw('name(owner(%M))', 'owner of player name', 'exact'),

    // --- home --- (different DBs have different home rooms — check it returns valid room)
    tw('type(home(%M))', 'home is ROOM type', 'exact'),

    // --- parent ---
    tw('name(parent(%G))', 'parent of gadget name', 'exact'),
    tw('parent(%W)', 'parent of widget (none)', 'exact'),

    // --- lparent --- (returns dbref list — count words instead)
    tw('words(lparent(%G))', 'lparent count', 'exact'),

    // --- zone ---
    tw('zone(%W)', 'zone of widget (none)', 'exact'),

    // --- controls ---
    tw('controls(%M,%W)', 'controls own thing', 'exact'),

    // --- con --- (our test room — widget is in it)
    tw('name(con(%C))', 'con of chamber name', 'exact'),

    // --- next --- (next item in room after widget)
    tw('name(next(%W))', 'next after widget name', 'exact'),

    // --- lexits --- (our test room has no exits)
    tw('lexits(%C)', 'lexits of chamber', 'exact'),

    // --- num --- (use locate instead of num(*name) — * prefix is player-only)
    t(`name(locate(${ME.go},${WIDGET_NAME},*))`, `name(locate(${ME.c},${WIDGET_NAME},*))`, 'locate by name', 'exact'),

    // --- valid ---
    tw('valid(%W)', 'valid thing ref', 'exact'),
    ts('valid(#-1)', 'valid nothing', 'exact'),
    ts('valid(#999999)', 'valid nonexistent', 'exact'),

    // --- hasattr ---
    tw('hasattr(%W,TESTATTR)', 'hasattr exists', 'exact'),
    tw('hasattr(%W,NOATTR)', 'hasattr missing', 'exact'),
    tw('hasattr(%G,TESTATTR)', 'hasattr on child (no inherit)', 'exact'),

    // --- hasattrp ---
    tw('hasattrp(%G,TESTATTR)', 'hasattrp inherited', 'exact'),
    tw('hasattrp(%G,NOATTR)', 'hasattrp missing', 'exact'),

    // --- get ---
    tw('get(%W/TESTATTR)', 'get attr value', 'exact'),
    tw('get(%W/NUMATTR)', 'get numeric attr', 'exact'),
    tw('get(%W/NOATTR)', 'get missing attr', 'exact'),

    // --- xget ---
    tw('xget(%W,TESTATTR)', 'xget attr value', 'exact'),

    // --- get_eval ---
    tw('get_eval(%W/UFUN)', 'get_eval attr', 'exact'),

    // --- v ---
    ts('v(n)', 'v(n) = name', 'exact'),

    // --- lattr ---
    tw('lattr(%W)', 'lattr of widget', 'exact'),
    tw('lattr(%W/TEST*)', 'lattr wildcard', 'exact'),

    // --- nattr ---
    tw('nattr(%W)', 'nattr count', 'exact'),

    // --- u ---
    tw('u(%W/UFUN,3,4)', 'u() call', 'exact'),
    tw('u(%W/UFUN,10,20)', 'u() call 2', 'exact'),

    // --- default ---
    tw('default(%W/TESTATTR,fallback)', 'default existing', 'exact'),
    tw('default(%W/NOATTR,fallback)', 'default missing', 'exact'),

    // --- edefault ---
    tw('edefault(%W/TESTATTR,fallback)', 'edefault existing', 'exact'),
    tw('edefault(%W/NOATTR,fallback)', 'edefault missing', 'exact'),

    // --- room ---
    tw('name(room(%W))', 'room of thing name', 'exact'),

    // --- rloc ---
    tw('name(rloc(%M,0))', 'rloc 0 name', 'exact'),

    // --- nearby ---
    tw('nearby(%M,%W)', 'nearby player to thing', 'exact'),

    // --- money/pennies --- (environmental diff, compare format not value)
    tw('isnum(money(%M))', 'money returns number', 'exact'),

    // --- objmem ---
    // objmem values differ between implementations — skip
    // tw('objmem(%W)', 'objmem', 'skip'),

    // --- locate ---
    t(`name(locate(${ME.go},${WIDGET_NAME},*))`, `name(locate(${ME.c},${WIDGET_NAME},*))`, 'locate thing name', 'exact'),
    // locate missing: both should return #-1 variant — check truthiness
    tw('strmatch(locate(%M,NoSuchObj99,*),#-*)', 'locate missing is #-*', 'exact'),

    // --- findable ---
    tw('findable(%M,%W)', 'findable own thing', 'exact'),

    // --- grep ---
    tw('grep(%W,*,hello)', 'grep attr content', 'exact'),
    tw('grepi(%W,*,HELLO)', 'grepi case insensitive', 'exact'),

    // --- setq/r ---
    ts('setq(0,test)[r(0)]', 'setq/r register', 'exact'),
    ts('setr(1,val1)', 'setr returns value', 'exact'),

    // --- objeval ---
    tw('objeval(%W,name(me))', 'objeval context', 'exact'),

    // --- lastcreate --- (check both 1-arg and 2-arg forms)
    // lastcreate: C 3.1 requires 2 args; wrap in name() to normalize dbrefs
    tw('name(lastcreate(%M, t))', 'lastcreate 2-arg name', 'exact'),

    // --- children --- (returns dbref — check name)
    tw('name(first(children(%W)))', 'children of widget name', 'exact'),
  ];

  let pass = 0, fail = 0;
  const failures = [];

  for (const [goExpr, cExpr, desc, mode] of tests) {
    const goCmd = `think [${goExpr}]`;
    const cCmd = `think [${cExpr}]`;

    let goVal = lastLine(await sendCmd(goSock, goCmd));
    let cVal = lastLine(await sendCmd(cSock, cCmd));

    let match = false;
    if (mode === 'exact') {
      match = (goVal === cVal);
    } else if (mode === 'dbref') {
      // Normalize dbrefs to names
      const goNorm = normalizeDbrefs(goVal, goMap);
      const cNorm = normalizeDbrefs(cVal, cMap);
      match = (goNorm === cNorm);
      if (!match) {
        goVal = `${goVal} → ${goNorm}`;
        cVal = `${cVal} → ${cNorm}`;
      }
    } else if (mode === 'bool') {
      match = (!!goVal === !!cVal);
    } else if (mode === 'skip') {
      pass++;
      continue;
    }

    if (match) {
      pass++;
    } else {
      fail++;
      failures.push({ desc, goExpr, go: goVal, c: cVal });
    }
  }

  const total = pass + fail;
  const pct = total > 0 ? Math.round((pass / total) * 100) : 0;

  console.log(`\n=== Object Function Audit ===`);
  console.log(`Total: ${total}  Pass: ${pass}  Fail: ${fail}  Match: ${pct}%`);

  if (failures.length > 0) {
    console.log(`\n--- Failures ---`);
    for (const f of failures) {
      console.log(`  ${f.desc}: ${f.goExpr}`);
      console.log(`    Go: ${JSON.stringify(f.go)}`);
      console.log(`     C: ${JSON.stringify(f.c)}`);
    }
  }

  // Cleanup: teleport back, destroy test objects and room
  console.log('\nCleaning up test objects...');
  await sendCmd(goSock, `@tel me=${goOrigLoc}`);
  await sendCmd(cSock, `@tel me=${cOrigLoc}`);
  await sendCmd(goSock, `@destroy ${goGadgetRef}`);
  await sendCmd(goSock, `@destroy ${goWidgetRef}`);
  await sendCmd(goSock, `@destroy ${goChamberRef}`);
  await sendCmd(cSock, `@destroy ${cGadgetRef}`);
  await sendCmd(cSock, `@destroy ${cWidgetRef}`);
  await sendCmd(cSock, `@destroy ${cChamberRef}`);

  goSock.write('QUIT\n');
  cSock.write('QUIT\n');
  setTimeout(() => process.exit(0), 500);
}

runTests().catch(e => { console.error(e); process.exit(1); });
