#!/usr/bin/env node
// CrystalMUSH Regression Test Suite
// Runs via WebSocket using JWT token auth.
//
// Usage:
//   TOKEN=$(curl -s -X POST http://localhost:8090/api/login \
//     -H 'Content-Type: application/json' \
//     -d '{"username":"Wizard","password":"YOUR_PASS"}' | jq -r .token)
//   node regression_test.js
//
// Or set TOKEN env var directly before running.

const WebSocket = require('ws');

const TOKEN = process.env.TOKEN;
if (!TOKEN) {
    console.error('Error: TOKEN environment variable required.');
    console.error('Get one via: curl -s -X POST http://localhost:8090/api/login \\');
    console.error('  -H "Content-Type: application/json" \\');
    console.error('  -d \'{"username":"Wizard","password":"YOUR_PASS"}\' | jq -r .token');
    process.exit(1);
}

const HOST = process.env.WS_HOST || `ws://localhost:8443/ws?token=${TOKEN}`;

let ws;
let buffer = '';
let testsPassed = 0;
let testsFailed = 0;
let testLog = [];

function log(msg) {
    console.log(msg);
    testLog.push(msg);
}

function pass(name) {
    testsPassed++;
    log(`  PASS: ${name}`);
}

function fail(name, detail) {
    testsFailed++;
    log(`  FAIL: ${name} -- ${detail}`);
}

function send(cmd) {
    ws.send(JSON.stringify({ type: 'command', command: cmd }));
}

function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function waitForOutput(ms = 1500) {
    await sleep(ms);
    const out = buffer;
    buffer = '';
    return out;
}

async function sendAndCapture(cmd, ms = 1500) {
    buffer = '';
    await sleep(200);
    buffer = '';
    send(cmd);
    return await waitForOutput(ms);
}

async function runTests() {
    log('=== CrystalMUSH Regression Test Suite ===\n');

    // Token auth — already logged in via query param
    await sleep(3000);
    let out = buffer;
    buffer = '';
    if (out.length < 5) {
        fail('Login', 'No response after connect');
        return finish();
    }
    pass('Connected via token auth');

    // ===== callfn() =====
    log('\n--- callfn() ---');
    out = await sendAndCapture('think [callfn(ADD,1,2,3)]');
    if (out.includes('6')) {
        pass('callfn(ADD,1,2,3) = 6');
    } else {
        fail('callfn(ADD,1,2,3)', `expected 6, got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('think [callfn(UCSTR,hello world)]');
    if (out.includes('HELLO WORLD')) {
        pass('callfn(UCSTR,hello world) = HELLO WORLD');
    } else {
        fail('callfn(UCSTR)', `expected HELLO WORLD, got: ${out.substring(0, 100)}`);
    }

    // ===== nextdbref() =====
    log('\n--- nextdbref() ---');
    out = await sendAndCapture('think [nextdbref()]');
    if (out.match(/#\d+/)) {
        pass('nextdbref() returns a dbref: ' + out.trim().split('\n').pop().trim());
    } else {
        fail('nextdbref()', `expected #NNN, got: ${out.substring(0, 100)}`);
    }

    // ===== JSON conversion functions =====
    log('\n--- JSON Conversion ---');

    out = await sendAndCapture('think [stringtojson(hello world)]');
    if (out.includes('"hello world"')) {
        pass('stringtojson(hello world)');
    } else {
        fail('stringtojson', `expected "hello world", got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('think [listtojson(red green blue)]');
    if (out.includes('["red","green","blue"]')) {
        pass('listtojson(red green blue)');
    } else {
        fail('listtojson', `expected ["red","green","blue"], got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('think [jsontolist([listtojson(a b c)])]');
    if (out.includes('a b c')) {
        pass('jsontolist roundtrip');
    } else {
        fail('jsontolist', `expected "a b c", got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('think [jsonescape(hello)]');
    if (out.includes('hello')) {
        pass('jsonescape(hello)');
    } else {
        fail('jsonescape', `got: ${out.substring(0, 100)}`);
    }

    // ===== Sensory commands =====
    log('\n--- Sensory Commands ---');

    out = await sendAndCapture('smell');
    if (out.includes("don't smell anything")) {
        pass('smell default message');
    } else {
        fail('smell default', `expected default msg, got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('@smell here=Fresh pine and cedar.');
    if (out.includes('Set.')) {
        pass('@smell setter');
    } else {
        fail('@smell setter', `expected Set., got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('smell');
    if (out.includes('Fresh pine and cedar')) {
        pass('smell shows attr');
    } else {
        fail('smell shows attr', `expected pine text, got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('touch');
    if (out.includes("don't feel anything")) {
        pass('touch default message');
    } else {
        fail('touch default', `expected default, got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('taste');
    if (out.includes("don't taste anything")) {
        pass('taste default message');
    } else {
        fail('taste default', `expected default, got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('listen');
    if (out.includes("don't hear anything")) {
        pass('listen default message');
    } else {
        fail('listen default', `expected default, got: ${out.substring(0, 100)}`);
    }

    // ===== @roomformat =====
    log('\n--- @roomformat ---');
    out = await sendAndCapture('@roomformat here=CUSTOM_FORMAT:[name(%0)]');
    if (out.includes('Set.')) {
        pass('@roomformat setter');
    } else {
        fail('@roomformat setter', `expected Set., got: ${out.substring(0, 100)}`);
    }

    out = await sendAndCapture('look');
    if (out.includes('CUSTOM_FORMAT:')) {
        pass('@roomformat renders custom output');
    } else {
        fail('@roomformat render', `expected CUSTOM_FORMAT:, got: ${out.substring(0, 200)}`);
    }

    // Clear roomformat
    await sendAndCapture('@roomformat here=');
    out = await sendAndCapture('look');
    if (!out.includes('CUSTOM_FORMAT:')) {
        pass('@roomformat cleared, normal look');
    } else {
        fail('@roomformat clear', 'still showing CUSTOM_FORMAT:');
    }

    // ===== @hook system =====
    log('\n--- @hook system ---');
    out = await sendAndCapture('@hook/list');
    if (out.includes('No hooks') || out.includes('hook') || out.includes('Hook')) {
        pass('@hook/list (no crash)');
    } else {
        fail('@hook/list', `unexpected: ${out.substring(0, 100)}`);
    }

    // ===== Multiple zones =====
    log('\n--- Multiple Zones ---');

    out = await sendAndCapture('@create Zone1');
    const z1Match = out.match(/#(\d+)/);
    out = await sendAndCapture('@create Zone2');
    const z2Match = out.match(/#(\d+)/);
    out = await sendAndCapture('@create ZoneTarget');
    const ztMatch = out.match(/#(\d+)/);

    if (z1Match && z2Match && ztMatch) {
        const z1 = z1Match[1], z2 = z2Match[1], zt = ztMatch[1];
        log(`  Zone1=#${z1} Zone2=#${z2} Target=#${zt}`);

        await sendAndCapture(`@chzone #${zt}=#${z1}`);

        out = await sendAndCapture(`@chzone/add #${zt}=#${z2}`);
        if (out.includes('Added') || out.includes('added') || out.includes('zone')) {
            pass('@chzone/add');
        } else {
            fail('@chzone/add', `got: ${out.substring(0, 100)}`);
        }

        out = await sendAndCapture(`think [zones(#${zt})]`);
        if (out.includes(`#${z1}`) && out.includes(`#${z2}`)) {
            pass('zones() returns both zones');
        } else {
            fail('zones()', `expected both zones, got: ${out.substring(0, 100)}`);
        }

        out = await sendAndCapture(`@chzone/remove #${zt}=#${z2}`);
        if (out.includes('Removed') || out.includes('removed') || out.includes('zone')) {
            pass('@chzone/remove');
        } else {
            fail('@chzone/remove', `got: ${out.substring(0, 100)}`);
        }

        out = await sendAndCapture(`think [zones(#${zt})]`);
        if (out.includes(`#${z1}`) && !out.includes(`#${z2}`)) {
            pass('zones() after remove');
        } else {
            fail('zones() after remove', `got: ${out.substring(0, 100)}`);
        }

        await sendAndCapture(`@destroy/override #${z1}`);
        await sendAndCapture(`@destroy/override #${z2}`);
        await sendAndCapture(`@destroy/override #${zt}`);
    } else {
        fail('zone object creation', 'could not create zone objects');
    }

    // ===== isinstance/irooms/ivehicle =====
    log('\n--- Instance functions ---');
    out = await sendAndCapture('think [isinstance(me)]');
    if (out.includes('0')) {
        pass('isinstance(me) = 0');
    } else {
        fail('isinstance(me)', `expected 0, got: ${out.substring(0, 100)}`);
    }

    // ===== Instance system =====
    log('\n--- Instance System ---');

    out = await sendAndCapture('@create Vehicle Template');
    const tmplMatch = out.match(/#(\d+)/);
    if (tmplMatch) {
        const tmpl = tmplMatch[1];
        log(`  Template: #${tmpl}`);

        out = await sendAndCapture('@dig Interior Cabin');
        const cabinMatch = out.match(/#(\d+)/);
        if (cabinMatch) {
            const cabin = cabinMatch[1];
            log(`  Cabin: #${cabin}`);

            await sendAndCapture(`@teleport #${cabin}=#${tmpl}`);
            await sendAndCapture(`@describe #${cabin}=Inside the vehicle.`);
            await sendAndCapture(`@set #${tmpl}=ENTER_OK`);

            out = await sendAndCapture(`@instance/create #${tmpl}=My Vehicle`);
            if (out.includes('Instance created') || out.includes('instance')) {
                pass('@instance/create');

                const instMatch = out.match(/My Vehicle\(#(\d+)\)/);
                if (instMatch) {
                    const inst = instMatch[1];
                    log(`  Instance: #${inst}`);

                    out = await sendAndCapture(`think [isinstance(#${inst})]`);
                    if (out.includes('1')) {
                        pass(`isinstance(#${inst}) = 1`);
                    } else {
                        fail('isinstance on instance', `expected 1, got: ${out.substring(0, 100)}`);
                    }

                    out = await sendAndCapture(`think [irooms(#${inst})]`);
                    if (out.match(/#\d+/)) {
                        pass('irooms() returns interior room');
                        const iroomMatch = out.match(/#(\d+)/);
                        if (iroomMatch) {
                            const iroom = iroomMatch[1];
                            out = await sendAndCapture(`think [ivehicle(#${iroom})]`);
                            if (out.includes(`#${inst}`)) {
                                pass(`ivehicle(#${iroom}) = #${inst}`);
                            } else {
                                fail('ivehicle()', `expected #${inst}, got: ${out.substring(0, 100)}`);
                            }
                        }
                    } else {
                        fail('irooms()', `expected dbref, got: ${out.substring(0, 100)}`);
                    }

                    // Drop instance and enter it
                    await sendAndCapture(`drop My Vehicle`);
                    await sleep(500);
                    out = await sendAndCapture(`enter My Vehicle`);
                    if (out.includes('You enter') || out.includes('Inside') || out.includes('Cabin')) {
                        pass('enter instance');
                    } else {
                        fail('enter instance', `got: ${out.substring(0, 200)}`);
                    }

                    out = await sendAndCapture('leave');
                    if (out.includes('You leave')) {
                        pass('leave instance');
                    } else {
                        fail('leave instance', `got: ${out.substring(0, 200)}`);
                    }

                    out = await sendAndCapture(`@instance/destroy #${inst}`);
                    if (out.includes('destroyed') || out.includes('Destroyed')) {
                        pass('@instance/destroy');
                    } else {
                        fail('@instance/destroy', `got: ${out.substring(0, 100)}`);
                    }
                } else {
                    log('  Could not parse instance dbref from: ' + out.substring(0, 200));
                }
            } else {
                fail('@instance/create', `got: ${out.substring(0, 200)}`);
            }
        }
        await sendAndCapture(`@destroy/override #${tmpl}`);
    }

    // ===== INSTANCE flag =====
    log('\n--- INSTANCE flag ---');
    out = await sendAndCapture('@create FlagTest');
    const ftMatch = out.match(/#(\d+)/);
    if (ftMatch) {
        const ft = ftMatch[1];
        out = await sendAndCapture(`@set #${ft}=INSTANCE`);
        if (out.includes('Set.')) {
            pass('@set INSTANCE flag');
        } else {
            fail('@set INSTANCE', `got: ${out.substring(0, 100)}`);
        }

        out = await sendAndCapture(`examine #${ft}`);
        if (out.includes('INSTANCE')) {
            pass('examine shows INSTANCE flag');
        } else {
            fail('examine INSTANCE', `expected INSTANCE in flags, got: ${out.substring(0, 200)}`);
        }

        await sendAndCapture(`@destroy/override #${ft}`);
    }

    // ===== locate() scope flags =====
    log('\n--- locate() scope flags ---');
    {
        // Create test room, exit with alias "1", and thing named "Carton 1"
        out = await sendAndCapture('@dig LocateTestRoom');
        const roomMatch = out.match(/#(\d+)/);
        if (roomMatch) {
            const room = roomMatch[1];
            // Create a "table" thing inside the room
            out = await sendAndCapture(`@create LocateTable`);
            const tableMatch = out.match(/#(\d+)/);
            // Create a "carton" thing
            out = await sendAndCapture(`@create Carton 1`);
            const cartonMatch = out.match(/#(\d+)/);

            // Teleport to test room so @open creates exit FROM the test room
            await sendAndCapture(`@tel me=#${room}`);
            out = await sendAndCapture(`@open 1st Exit;1st;1`);
            const exitMatch = out.match(/#(\d+)/);
            // Return to starting room
            await sendAndCapture('home');

            if (tableMatch && cartonMatch && exitMatch) {
                const table = tableMatch[1];
                const carton = cartonMatch[1];
                const exit = exitMatch[1];

                // Move table and carton into the room
                await sendAndCapture(`@tel #${table}=#${room}`);
                await sendAndCapture(`@tel #${carton}=#${room}`);

                // locate(table, 1, n) — neighbors only — should find Carton 1, NOT the exit
                out = await sendAndCapture(`think [locate(#${table}, 1, n)]`);
                if (out.includes(`#${carton}`)) {
                    pass('locate(table, 1, n) finds Carton 1 not exit');
                } else {
                    fail('locate(table, 1, n)', `expected #${carton} (Carton 1), got: ${out.trim()}`);
                }

                // locate(table, 1, e) — exits only — should find the exit
                out = await sendAndCapture(`think [locate(#${table}, 1, e)]`);
                if (out.includes(`#${exit}`)) {
                    pass('locate(table, 1, e) finds exit');
                } else {
                    fail('locate(table, 1, e)', `expected #${exit} (exit), got: ${out.trim()}`);
                }

                // locate(table, 1, ne) — neighbors + exits — exit wins (exact alias)
                out = await sendAndCapture(`think [locate(#${table}, 1, ne)]`);
                if (out.includes(`#${exit}`)) {
                    pass('locate(table, 1, ne) exact exit wins over prefix carton');
                } else {
                    fail('locate(table, 1, ne)', `expected #${exit} (exit), got: ${out.trim()}`);
                }

                // locate(table, Carton, n) — should find carton by prefix
                out = await sendAndCapture(`think [locate(#${table}, Carton, n)]`);
                if (out.includes(`#${carton}`)) {
                    pass('locate(table, Carton, n) finds carton');
                } else {
                    fail('locate(table, Carton, n)', `expected #${carton}, got: ${out.trim()}`);
                }

                // Cleanup
                await sendAndCapture(`@destroy/override #${exit}`);
                await sendAndCapture(`@destroy/override #${carton}`);
                await sendAndCapture(`@destroy/override #${table}`);
            }
            await sendAndCapture(`@destroy/override #${room}`);
        }
    }

    // ===== @tel departure messages for non-player objects =====
    log('\n--- @tel/@dest departure messages ---');
    {
        out = await sendAndCapture('@dig Test Departure Room');
        let dRoom = out.match(/#(\d+)/)?.[1];
        if (dRoom) {
            out = await sendAndCapture(`@create DepartObj`);
            let dThing = out.match(/#(\d+)/)?.[1];
            if (dThing) {
                // Place thing in test room, teleport there
                await sendAndCapture(`@tel #${dThing}=#${dRoom}`);
                await sendAndCapture(`@tel me=#${dRoom}`);

                // @tel thing away — should see "has left."
                out = await sendAndCapture(`@tel #${dThing}=#0`);
                if (out.includes('has left')) {
                    pass('@tel non-player: departure message');
                } else {
                    fail('@tel non-player: departure message', `expected "has left", got: ${out.trim()}`);
                }

                // Bring it back and @dest it
                await sendAndCapture(`@tel #${dThing}=#${dRoom}`);
                out = await sendAndCapture(`@dest #${dThing}`);
                if (out.includes('has left')) {
                    pass('@dest non-player: departure message');
                } else {
                    fail('@dest non-player: departure message', `expected "has left", got: ${out.trim()}`);
                }
            }
            await sendAndCapture(`@tel me=home`);
            await sendAndCapture(`@destroy/override #${dRoom}`);
        }
    }

    // ===== Help entries =====
    log('\n--- Help entries ---');
    const helpTopics = [
        '@roomformat', 'smell', '@hook', 'callfn()', 'nextdbref()', 'zones()',
        'isinstance()', 'irooms()', 'ivehicle()', '@instance', 'mogrifier',
        'stringtojson()', 'listtojson()', 'jsontolist()', 'jsonescape()'
    ];
    for (const topic of helpTopics) {
        out = await sendAndCapture(`help ${topic}`, 1000);
        if (out.includes('No entry') || out.includes('no help') || out.length < 30) {
            fail(`help ${topic}`, 'no help entry found');
        } else {
            pass(`help ${topic}`);
        }
    }

    // Clean up smell attr
    await sendAndCapture('@smell here=');

    finish();
}

function finish() {
    log(`\n=== Results: ${testsPassed} passed, ${testsFailed} failed ===`);
    ws.close();
    process.exit(testsFailed > 0 ? 1 : 0);
}

// Connect
ws = new WebSocket(HOST);
ws.on('open', () => {
    runTests();
});
ws.on('message', (data) => {
    try {
        const msg = JSON.parse(data.toString());
        if (msg.text) {
            buffer += msg.text + '\n';
        }
    } catch (e) {
        buffer += data.toString() + '\n';
    }
});
ws.on('error', (err) => {
    console.error('WebSocket error:', err.message);
    process.exit(1);
});

setTimeout(() => {
    log('\n=== TIMEOUT after 180s ===');
    finish();
}, 180000);
