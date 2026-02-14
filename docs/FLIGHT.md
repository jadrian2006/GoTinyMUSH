# Flight & Navigation System

GoTinyMUSH includes a complete 3D flight and navigation system built as softcode functions. It provides a 4-quadrant grid coordinate system, 32-point compass headings, navigation projection with drift/entropy, and multi-object tactical calculations for combat and intercept scenarios.

These functions are designed for MUSHes that feature vehicle combat, space flight, naval warfare, or any game that needs objects moving through a coordinate grid. The system is entirely softcode-driven — the server provides the math primitives, and builders wire them together with `@trigger`, `@wait`, and attributes to create flight loops.

---

## Table of Contents

- [Grid Coordinate System](#grid-coordinate-system)
  - [Quadrants](#quadrants)
  - [Letter Coordinates (X-Axis)](#letter-coordinates-x-axis)
  - [Number Coordinates (Y-Axis)](#number-coordinates-y-axis)
  - [Altitude (Z-Axis)](#altitude-z-axis)
  - [Grid Address Format](#grid-address-format)
  - [Absolute Coordinates](#absolute-coordinates)
  - [Customizing Grid Bounds](#customizing-grid-bounds)
- [Heading System](#heading-system)
  - [32-Point Compass](#32-point-compass)
  - [Heading Reference Table](#heading-reference-table)
- [Function Reference](#function-reference)
  - [Heading Functions](#heading-functions)
  - [Grid Coordinate Functions](#grid-coordinate-functions)
  - [Navigation Functions](#navigation-functions)
  - [Drift / Entropy Functions](#drift--entropy-functions)
  - [Tactical Functions](#tactical-functions)
- [Building a Flight System](#building-a-flight-system)
  - [Basic Flight Loop](#basic-flight-loop)
  - [Autopilot](#autopilot)
  - [Radar / Detection](#radar--detection)
  - [Combat Engagement](#combat-engagement)

---

## Grid Coordinate System

The flight grid divides 2D space into four quadrants centered on an origin point, with an independent altitude (Z) axis. Every position in the grid has a human-readable address like `EL-453-NE` and a corresponding pair of absolute X,Y coordinates used internally for math.

### Quadrants

The grid is divided into four quadrants based on compass directions:

```
                  N
                  |
       NW         |         NE
    (-x, +y)     |      (+x, +y)
                  |
  W ──────────── 0,0 ──────────── E
                  |
    (-x, -y)     |      (+x, -y)
       SW         |         SE
                  |
                  S
```

- **NE** (Northeast): Positive X, Positive Y
- **NW** (Northwest): Negative X, Positive Y
- **SE** (Southeast): Positive X, Negative Y
- **SW** (Southwest): Negative X, Negative Y

### Letter Coordinates (X-Axis)

Within each quadrant, the X-axis is represented by a two-letter code from `AA` to `ZZ`, giving **676 positions** (26 x 26) per quadrant. Letters increase from West to East.

```
AA AB AC ... AZ BA BB ... ZY ZZ
 0  1  2     25 26 27    674 675
```

For example: `AA` = position 0, `AZ` = position 25, `BA` = position 26, `EL` = position 115, `ZZ` = position 675.

### Number Coordinates (Y-Axis)

The Y-axis is represented by a number from `0` to `999`, giving **1000 positions** per quadrant. Numbers increase from South to North.

### Altitude (Z-Axis)

Altitude is tracked as a separate numeric value, independent of the grid quadrant system. It is stored as the third component of a position vector (e.g., `100 200 50` means X=100, Y=200, altitude=50). The altitude range is game-defined — the functions impose no limits.

### Grid Address Format

Grid addresses use the format `LL-NNN-QQ`:

| Component | Description | Range |
|---|---|---|
| `LL` | Two-letter X coordinate | `AA` to `ZZ` (676 values) |
| `NNN` | Numeric Y coordinate | `0` to `999` |
| `QQ` | Quadrant | `NE`, `NW`, `SE`, `SW` |

**Examples:**
- `AA-0-NE` — Origin corner of the NE quadrant (absolute: 0, 0)
- `EL-453-NE` — Deep in the NE quadrant (absolute: 115, 453)
- `MZ-500-NW` — Middle of the NW quadrant (absolute: -338, 500)
- `AA-0-SW` — Origin corner of the SW quadrant (absolute: -676, -1000)

Both dash-delimited (`EL-453-NE`) and space-delimited (`EL 453 NE`) formats are accepted by all grid functions.

### Absolute Coordinates

Internally, all math operates on absolute X,Y coordinates with the origin at the center of the map. The conversion formulas are:

**Grid to Absolute:**

| Quadrant | X | Y |
|---|---|---|
| NE | `letter_pos` | `number` |
| NW | `letter_pos - 676` | `number` |
| SE | `letter_pos` | `number - 1000` |
| SW | `letter_pos - 676` | `number - 1000` |

**Absolute to Grid:**

| Condition | Quadrant | Letter Position | Number |
|---|---|---|---|
| X >= 0, Y >= 0 | NE | `x` | `y` |
| X < 0, Y >= 0 | NW | `x + 676` | `y` |
| X >= 0, Y < 0 | SE | `x` | `y + 1000` |
| X < 0, Y < 0 | SW | `x + 676` | `y + 1000` |

The total map spans **1352 units** on the X-axis (676 per side) and **2000 units** on the Y-axis (1000 per side).

### Customizing Grid Bounds

The grid constants are defined in `flight.go`:

```go
const gridLetters = 676   // 26*26 = AA to ZZ
const gridNumbers = 1000  // 0 to 999
```

To create a smaller or larger grid, adjust these values. The letter system (AA-ZZ) is fixed at 676 positions, but `gridNumbers` can be changed freely. For a 500x500 grid per quadrant, set `gridNumbers = 500`. All conversion functions (`gridToAbs`, `absToGrid`, `parseGridLoc`) use these constants, so changes propagate automatically.

---

## Heading System

### 32-Point Compass

Headings use a 32-point compass system where **0 = East** and values increase **counterclockwise**. Each heading step represents 11.25 degrees. This convention matches standard mathematical angle measurement (positive angles rotate from the X-axis toward Y).

```
                 N (8)
                 |
     NW (12) ----+---- NE (4)
                 |
     W (16) ----+---- E (0)
                 |
     SW (20) ----+---- SE (28)
                 |
                 S (24)
```

### Heading Reference Table

| Heading | Direction | Degrees | Exact Name |
|---|---|---|---|
| 0 | E | 0.00 | E |
| 1 | EbN | 11.25 | EbN |
| 2 | ENE | 22.50 | ENE |
| 3 | NEbE | 33.75 | NEbE |
| 4 | NE | 45.00 | NE |
| 5 | NEbN | 56.25 | NEbN |
| 6 | NNE | 67.50 | NNE |
| 7 | NbE | 78.75 | NbE |
| 8 | N | 90.00 | N |
| 9 | NbW | 101.25 | NbW |
| 10 | NNW | 112.50 | NNW |
| 11 | NWbN | 123.75 | NWbN |
| 12 | NW | 135.00 | NW |
| 13 | NWbW | 146.25 | NWbW |
| 14 | WNW | 157.50 | WNW |
| 15 | WbN | 168.75 | WbN |
| 16 | W | 180.00 | W |
| 17 | WbS | 191.25 | WbS |
| 18 | WSW | 202.50 | WSW |
| 19 | SWbW | 213.75 | SWbW |
| 20 | SW | 225.00 | SW |
| 21 | SWbS | 236.25 | SWbS |
| 22 | SSW | 247.50 | SSW |
| 23 | SbW | 258.75 | SbW |
| 24 | S | 270.00 | S |
| 25 | SbE | 281.25 | SbE |
| 26 | SSE | 292.50 | SSE |
| 27 | SEbS | 303.75 | SEbS |
| 28 | SE | 315.00 | SE |
| 29 | SEbE | 326.25 | SEbE |
| 30 | ESE | 337.50 | ESE |
| 31 | EbS | 348.75 | EbS |

---

## Function Reference

All examples use `think [function(...)]` syntax as entered in-game. Return values shown after `->`.

### Heading Functions

#### `hvec(<heading>)`

Converts a heading (0-31) to its unit direction vector.

```
think [hvec(0)]     -> 1 0         (East)
think [hvec(4)]     -> 0.71 0.71   (Northeast)
think [hvec(8)]     -> 0 1         (North)
think [hvec(16)]    -> -1 0        (West)
think [hvec(24)]    -> 0 -1        (South)
```

#### `hdelta(<heading1>, <heading2>)`

Returns the shortest turn between two headings. Positive values mean counterclockwise (left), negative means clockwise (right). Range: -16 to +16.

```
think [hdelta(0, 4)]    -> 4    (turn left 4 steps from E to NE)
think [hdelta(0, 28)]   -> -4   (turn right 4 steps from E to SE)
think [hdelta(2, 30)]   -> -4   (shortest path wraps around)
think [hdelta(0, 16)]   -> 16   (half turn, maximum delta)
```

This is useful for smooth turning — if `hdelta` returns +3, turn left 3 steps over 3 ticks rather than snapping instantly.

#### `hname(<heading>[, <exact>])`

Converts a heading to a compass direction name. With no second argument (or 0), returns a 16-point name. With `exact` = 1, returns the precise 32-point name.

```
think [hname(0)]        -> E
think [hname(4)]        -> NE
think [hname(1)]        -> ENE     (16-point: rounds to nearest)
think [hname(1, 1)]     -> EbN     (32-point: "East by North")
think [hname(7, 1)]     -> NbE     (32-point: "North by East")
```

#### `h2deg(<heading>)`

Converts a heading to degrees (0-360).

```
think [h2deg(0)]     -> 0       (East)
think [h2deg(8)]     -> 90      (North)
think [h2deg(16)]    -> 180     (West)
think [h2deg(24)]    -> 270     (South)
think [h2deg(4)]     -> 45      (Northeast)
```

#### `deg2h(<degrees>)`

Converts degrees to the nearest heading (0-31). Rounds to the closest step.

```
think [deg2h(0)]      -> 0    (East)
think [deg2h(90)]     -> 8    (North)
think [deg2h(45)]     -> 4    (Northeast)
think [deg2h(100)]    -> 9    (closest to NbW)
think [deg2h(-90)]    -> 24   (negative wraps: South)
```

#### `vec2h(<x>, <y>)`

Converts a direction vector to the nearest heading (0-31).

```
think [vec2h(1, 0)]     -> 0    (East)
think [vec2h(0, 1)]     -> 8    (North)
think [vec2h(1, 1)]     -> 4    (Northeast)
think [vec2h(-1, 0)]    -> 16   (West)
think [vec2h(3, 1)]     -> 1    (mostly East, slightly North)
```

---

### Grid Coordinate Functions

#### `gridabs(<grid_location>)` or `gridabs(<letters>, <number>, <quadrant>)`

Converts a grid address to absolute X,Y coordinates. Accepts either a single combined string or three separate arguments.

```
think [gridabs(AA-0-NE)]        -> 0 0
think [gridabs(EL-453-NE)]      -> 115 453
think [gridabs(AA-0-NW)]        -> -676 0
think [gridabs(AA-0-SE)]        -> 0 -1000
think [gridabs(AA-0-SW)]        -> -676 -1000
think [gridabs(EL, 453, NE)]    -> 115 453     (3-arg form)
```

#### `absgrid(<x>, <y>)`

Converts absolute coordinates back to a grid address string.

```
think [absgrid(0, 0)]           -> AA-0-NE
think [absgrid(115, 453)]       -> EL-453-NE
think [absgrid(-676, 0)]        -> AA-0-NW
think [absgrid(0, -1000)]       -> AA-0-SE
think [absgrid(-338, 500)]      -> MZ-500-NW
```

#### `griddist(<location1>, <location2>)`

Returns the 2D Euclidean distance between two grid locations. Altitude is not considered.

```
think [griddist(AA-0-NE, AA-10-NE)]           -> 10           (10 units north)
think [griddist(AA-0-NE, ZZ-999-NE)]          -> 1206.09      (corner to corner of NE)
think [griddist(AA-0-NW, AA-0-NE)]            -> 676          (across the prime meridian)
think [griddist(AA-500-NE, AA-500-NW)]         -> 676          (same latitude, opposite quadrants)
```

#### `gridcourse(<from>, <to>)`

Returns the heading and distance from one grid location to another as `"heading distance"`.

```
think [gridcourse(AA-0-NE, EL-453-NE)]    -> 6 469.33    (heading NNE, distance 469)
think [gridcourse(AA-0-NE, AA-100-NE)]    -> 8 100       (due North, 100 units)
think [gridcourse(AA-0-NE, ZZ-0-NE)]      -> 0 675       (due East, 675 units)
```

The first value is the heading (0-31) to steer, and the second is the straight-line distance.

---

### Navigation Functions

#### `gridnav(<position>, <heading>, <speed>[, <climb>[, <drift>]])`

Projects a new 3D position given current position, heading, speed, optional climb rate, and optional drift. This is the core function called each tick to move an object.

**Arguments:**
- `position` — Current position as `"x y z"` (space-delimited vector)
- `heading` — Current heading (0-31)
- `speed` — Speed in grid units per tick
- `climb` — (Optional) Altitude change per tick. Positive = climb, negative = dive.
- `drift` — (Optional) Maximum random perturbation per axis per tick. Each axis gets an independent random offset in `[-drift, +drift]`.

**Basic movement:**
```
think [gridnav(100 200 50, 0, 10)]          -> 110 200 50       (move East at speed 10)
think [gridnav(100 200 50, 8, 10)]          -> 100 210 50       (move North at speed 10)
think [gridnav(100 200 50, 4, 10)]          -> 107.07 207.07 50 (move NE at speed 10)
```

**With climb:**
```
think [gridnav(100 200 50, 0, 10, 5)]       -> 110 200 55       (move East, climb 5)
think [gridnav(100 200 50, 8, 10, -3)]      -> 100 210 47       (move North, dive 3)
```

**With drift (results vary due to randomness):**
```
think [gridnav(100 200 50, 0, 10, 0, 3)]    -> 112.1 198.7 51.4  (move East + random ±3)
think [gridnav(100 200 50, 0, 10, 5, 2)]    -> 111.3 201.2 56.8  (move East, climb 5, drift ±2)
```

---

### Drift / Entropy Functions

These functions add controlled randomness to positions and vectors, simulating turbulence, sensor noise, weapon scatter, or environmental effects.

#### `vrand(<max_magnitude>[, <dimensions>])`

Generates a random direction vector with magnitude between 0 and `max_magnitude`. The direction is uniformly random on the unit sphere; magnitude is uniformly distributed in `[0, max]`. Default is 3 dimensions.

```
think [vrand(5)]        -> -2.31 1.74 3.12    (random 3D vector, magnitude 0-5)
think [vrand(10)]       -> 4.56 -7.23 2.89    (random 3D vector, magnitude 0-10)
think [vrand(5, 2)]     -> 3.21 -1.45          (random 2D vector)
```

#### `vrandc(<max_per_component>)`

Generates a per-component random vector where each axis is independently randomized in `[-max, +max]`. The argument is a vector specifying the maximum for each component. This is useful when different axes need different drift ranges.

```
think [vrandc(5 5 1)]       -> 2.3 -4.1 0.7    (XY drift ±5, altitude drift ±1)
think [vrandc(10 10 0)]     -> -3.4 7.2 0       (XY drift only, no altitude)
think [vrandc(3)]           -> 1.2               (1D random in [-3, +3])
```

#### `drift(<position>, <max_drift>)`

Applies random drift to a position vector. `max_drift` can be a single scalar (same for all axes) or a vector (per-component maximum). Returns the new drifted position.

```
think [drift(100 200 50, 3)]           -> 101.7 198.4 51.2   (uniform ±3 on all axes)
think [drift(100 200 50, 5 5 1)]       -> 103.2 197.8 50.4   (±5 on XY, ±1 on altitude)
think [drift(100 200 50, 10 10 0)]     -> 107.3 194.1 50     (XY drift, altitude stable)
```

---

### Tactical Functions

These functions support multi-object scenarios: detecting other craft, computing engagement geometry, and solving intercept problems.

#### `bearing(<position1>, <position2>)`

Returns the heading (0-31) that an object at position1 would need to fly to face position2. This is a 2D calculation (ignores Z).

```
think [bearing(100 200, 300 400)]          -> 4    (NE)
think [bearing(100 200, 100 400)]          -> 8    (due North)
think [bearing(100 200, 300 200)]          -> 0    (due East)
think [bearing(100 200, 0 100)]            -> 20   (SW)
```

**Typical use:** On a sensor display, show the compass direction to each detected contact.

#### `pitch(<position1>, <position2>)`

Returns the vertical angle in degrees from position1 to position2. Positive = target is above, negative = target is below. Range: -90 to +90. Requires 3D positions.

```
think [pitch(100 200 50, 100 200 80)]     -> 90     (directly above)
think [pitch(100 200 50, 100 200 20)]     -> -90    (directly below)
think [pitch(100 200 50, 200 200 100)]    -> 26.57  (above and ahead)
think [pitch(100 200 50, 200 300 50)]     -> 0      (same altitude)
```

**Typical use:** Determine if a target is within weapon firing arc (e.g., guns limited to ±30 degrees pitch).

#### `closing(<pos1>, <heading1>, <speed1>, <pos2>, <heading2>, <speed2>)`

Calculates the closing rate between two moving objects in distance units per tick. **Positive = getting closer**, negative = separating.

```
think [closing(0 0, 0, 10, 100 0, 16, 10)]     -> 20    (head-on, both at speed 10)
think [closing(0 0, 0, 10, 100 0, 0, 10)]      -> -10   (both flying East, separating)
think [closing(0 0, 0, 10, 100 0, 16, 5)]       -> 15    (approaching, obj2 slower)
think [closing(0 0, 8, 10, 0 100, 24, 10)]      -> 20    (head-on, N/S axis)
```

**Typical use:** Determine threat level of incoming contacts. A high positive closing rate means a fast approach; negative means the contact is moving away.

#### `relvel(<heading1>, <speed1>, <heading2>, <speed2>)`

Returns the relative velocity vector of object 2 as seen from object 1. The result is `"dx dy"` — the per-tick displacement of obj2 relative to obj1.

```
think [relvel(0, 10, 16, 10)]     -> -20 0     (head-on along X axis)
think [relvel(0, 10, 0, 10)]      -> 0 0       (same heading and speed = stationary)
think [relvel(0, 10, 0, 15)]      -> 5 0       (same heading, obj2 faster)
think [relvel(0, 10, 8, 10)]      -> -10 10    (obj2 going North while obj1 goes East)
```

**Typical use:** Display relative motion on a tactical scope, or determine if a contact is crossing left-to-right or approaching head-on.

#### `eta(<position1>, <heading>, <speed>, <position2>)`

Estimates the number of ticks to reach position2 from position1 at the given heading and speed, assuming the target is stationary. Returns `-1` if moving away or perpendicular to the target.

```
think [eta(0 0, 0, 10, 100 0)]      -> 10     (100 units East at speed 10)
think [eta(0 0, 0, 10, 0 100)]      -> -1     (heading East but target is North)
think [eta(0 0, 4, 14.14, 100 100)] -> 10     (heading NE toward NE target)
think [eta(0 0, 0, 10, 50 10)]      -> 5.1    (mostly East, close enough)
```

**Typical use:** "Target at bearing 4, distance 200, ETA 20 ticks."

#### `intercept(<pos1>, <speed1>, <pos2>, <heading2>, <speed2>)`

Calculates the optimal heading (0-31) for object 1 to intercept a moving object 2. Uses binary search to solve the pursuit geometry — finding the future point where obj2 will be when obj1 can reach it. Returns `-1` if no intercept is possible (e.g., target is faster and moving away).

```
think [intercept(0 0, 15, 100 0, 8, 10)]       -> 6    (fly NNE to intercept)
think [intercept(0 0, 10, 100 100, 16, 5)]      -> 4    (fly NE to catch westbound target)
think [intercept(0 0, 10, 100 0, 0, 10)]        -> 0    (target fleeing East, chase directly)
think [intercept(0 0, 10, 50 0, 8, 10)]         -> 5    (lead the target going North)
```

**Typical use:** Autopilot intercept course. "Setting intercept course, heading NNE, ETA 12 ticks."

---

## Building a Flight System

The functions above provide the math primitives. Here are patterns for wiring them together in softcode to build a complete flight system.

### Basic Flight Loop

A flight loop runs on a timer, updating each craft's position every tick:

```
@@ Attributes on each craft object:
@@ FPOS   - current position "x y z"
@@ FHEAD  - current heading (0-31)
@@ FSPEED - current speed (0-40)
@@ FCLIMB - climb rate per tick
@@ FDRIFT - drift per tick (0 = none)

&FLIGHT_TICK me=
  @dolist [search(TYPE=THING EVAL=\[gt(v(FSPEED,##),0)\])]=
    @trigger ##/FLIGHT_MOVE;

&FLIGHT_MOVE me=
  think [setq(0, gridnav(v(FPOS), v(FHEAD), v(FSPEED), v(FCLIMB), v(FDRIFT)))];
  &FPOS me=[r(0)];
  @trigger me/FLIGHT_REPORT;

&FLIGHT_REPORT me=
  @pemit [owner(me)]=
    Loc: [absgrid(first(v(FPOS)), rest(first(rest(v(FPOS)))))]
    Heading: [hname(v(FHEAD))]([v(FHEAD)])
    Alt: [last(v(FPOS))]
    Speed: [v(FSPEED)];

@@ Start the loop (1 tick = ~1 second with @wait)
&START_FLIGHT me=@wait 1=@trigger me/FLIGHT_TICK; @trigger me/START_FLIGHT;
```

### Autopilot

Set a destination and let the craft adjust heading each tick to steer toward it:

```
&CMD_SETCOURSE me=$set course *:
  think [setq(0, gridabs(%0))];
  @switch [r(0)]=
    #-1*, {@pemit %#=Invalid grid location: %0},
    {
      &FDEST me=[r(0)];
      @pemit %#=Course set to %0 ([r(0)]).;
    };

&AUTOPILOT me=
  @switch [hasattr(me, FDEST)]=1,{
    think [setq(0, bearing(v(FPOS), v(FDEST)))];
    think [setq(1, hdelta(v(FHEAD), r(0)))];
    @switch [gt(abs(r(1)), 0)]=1,{
      @@ Turn 1 step toward target per tick
      think [setq(2, switch(sign(r(1)), 1, 1, -1, -1, 0))];
      &FHEAD me=[mod(add(v(FHEAD), r(2), 32), 32)];
    };
    @@ Check arrival (within 1 unit)
    think [setq(3, dist(v(FPOS), v(FDEST)))];
    @switch [lte(r(3), 1)]=1,{
      &FSPEED me=0;
      &FDEST me=;
      @pemit [owner(me)]=Arrived at destination.;
    };
  };
```

### Radar / Detection

Scan for other craft within sensor range:

```
&CMD_SCAN me=$scan:
  think [setq(0, v(FPOS))];
  think [setq(9,)];
  @dolist [search(TYPE=THING EVAL=\[hasattr(##,FPOS)\])]=
    @switch ##=[num(me)],,{
      think [setq(1, v(FPOS, ##))];
      think [setq(2, dist(first(r(0)) first(rest(r(0))), first(r(1)) first(rest(r(1)))))];
      @switch [lte(r(2), 500)]=1,{
        think [setq(3, bearing(r(0), r(1)))];
        think [setq(4, pitch(r(0), r(1)))];
        think [setq(5, closing(r(0), v(FHEAD), v(FSPEED), r(1), v(FHEAD, ##), v(FSPEED, ##)))];
        @pemit %#=
          [rjust(name(##), 15)]
          Brg:[rjust(hname(r(3)),3)]
          Dst:[rjust(r(2),7)]
          Pit:[rjust(r(4),4)]
          Cls:[rjust(r(5),5)];
      };
    };
```

**Sample output:**
```
  Enemy Viper   Brg: NE  Dst:  234.5  Pit:  12  Cls: 18.3
  Freighter     Brg:  W  Dst:  891.2  Pit:  -3  Cls: -5.0
```

### Combat Engagement

Calculate an intercept course and fire when in range:

```
&CMD_INTERCEPT me=$intercept *:
  think [setq(0, pmatch(%0))];
  @switch [valid(r(0))]=0, {@pemit %#=Target not found.},
  {
    think [setq(1, v(FPOS, r(0)))];
    think [setq(2, intercept(v(FPOS), v(FSPEED), r(1), v(FHEAD, r(0)), v(FSPEED, r(0))))];
    @switch [r(2)]=-1,
      {@pemit %#=Cannot intercept — target is too fast or moving away.},
      {
        &FHEAD me=[r(2)];
        think [setq(3, eta(v(FPOS), r(2), v(FSPEED), r(1)))];
        @pemit %#=Intercept course set: heading [hname(r(2))] ([r(2)]), ETA [r(3)] ticks.;
      };
  };

&CMD_FIRE me=$fire:
  think [setq(0, v(FTARGET))];
  think [setq(1, dist(first(v(FPOS)) first(rest(v(FPOS))), first(v(FPOS,r(0))) first(rest(v(FPOS,r(0))))))];
  @switch [lte(r(1), 50)]=1,
    {@pemit %#=Weapons fire! Target at range [r(1)].},
    {@pemit %#=Target out of range ([r(1)] > 50).};
```

---

## Summary

| Category | Functions | Count |
|---|---|---|
| Heading | `hvec`, `hdelta`, `hname`, `h2deg`, `deg2h`, `vec2h` | 6 |
| Grid Coordinates | `gridabs`, `absgrid`, `griddist`, `gridcourse` | 4 |
| Navigation | `gridnav` | 1 |
| Drift / Entropy | `vrand`, `vrandc`, `drift` | 3 |
| Tactical | `bearing`, `pitch`, `closing`, `relvel`, `eta`, `intercept` | 6 |
| **Total** | | **20** |

All 20 functions are implemented in `pkg/eval/functions/flight.go` and registered in `pkg/eval/functions/register.go`.
