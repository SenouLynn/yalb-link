# Electric power and mission fixtures

These are **independent of the book**. The primary reference's powerplant chapter
sizes a piston engine and its mission analysis burns fuel; neither produces a
number an electric RC aircraft can be checked against. Every value below was
computed at 50 significant digits from the relations this package implements,
and none of it is a recommended default for any real aircraft.

> **To revisit: what the fuel-fraction method's electric analogue actually is.**
> The obvious substitution — swap the fuel-weight fraction for a
> state-of-charge fraction — does not carry over, and it is worth writing down
> why so the idea is not re-adopted by accident. The book's weight fractions
> matter because the aircraft gets *lighter*: `W` falls through the cruise, so
> `CL = W/(qS)` falls, `L/D` moves along the polar, and the power required
> changes. An electric aircraft's mass is constant, so that entire mechanism is
> absent and a depletion fraction changes nothing on the airframe side.
>
> What genuinely does change through an electric flight is on the *supply*
> side: pack terminal voltage falls with state of charge and sags further under
> load through the cells' internal resistance, so at a fixed throttle the
> available power, the rpm and the thrust all fall as the flight goes on. That
> is a propulsion-availability model against state of charge, not a weight
> model, and it would touch the capability points rather than the segment
> energies — a capability point would become a curve in state of charge instead
> of a single condition. There is a second, smaller effect worth capturing at
> the same time: constant electrical power at a sagging voltage means rising
> current, so a peak-current check that passes at 100% charge can fail at 20%.
>
> Doing it needs pack evidence this package does not ask for today: a discharge
> curve or an internal resistance and an open-circuit voltage against state of
> charge, both with their own basis and evidence grade, and it needs the
> propeller operating-point model that is already recorded as unsupported,
> because falling voltage moves the propeller along its own curve. Until that
> evidence exists the `usable fraction` stands in for the whole family of
> effects, which is why it is documented as covering "voltage sag, cell balance
> and the state of charge the pack is not taken below" rather than as a clean
> energy accounting term. Recorded 2026-09-07 as deferred work, not as an
> implemented adaptation.

## The fixture aircraft

| Quantity | Value | Note |
|---|---|---|
| all-up mass `m` | 2.5 kg | synthetic |
| reference area `S` | 0.40 m² | synthetic |
| plan-view aspect ratio `A` | 8 | synthetic |
| air density `rho` | 1.225 kg/m³ | ISA sea level |
| `CD0` | 0.035 | synthetic fixture assumption, aircraft level, clean |
| Oswald `e` | 0.85 | synthetic fixture assumption |
| polar validity `CL` | −0.2 to 1.1 | synthetic fixture assumption |
| case `CLmax` | 1.2 | synthetic fixture assumption |
| chain efficiency `eta_total` | 0.55 | synthetic; propeller, motor and ESC together |
| auxiliary continuous draw | 8 W | synthetic, pack side |

Derived once, shared by every case below:

```
K = 1/(pi * A * e) = 0.04681027737996921640261287
W = m * g          = 24.516625 N            (g = 9.80665 m/s^2)
```

## Level cruise, 16 m/s, still air

`n = 1`, `gamma = 0`, so `L = W`.

| Quantity | Value |
|---|---|
| `q = rho V²/2` | 156.8 Pa |
| `CL = L/(q S)` | 0.3908900669642857142857143 |
| `CD = CD0 + K CL²` | 0.0421523784130521282030199 |
| `L/D` | 9.273262427423310097793621 |
| `D = q S CD` | 2.643797174066629480893408 N |
| `T = D` (level) | 2.643797174066629480893408 N |
| `P_useful = T V` | 42.30075478506607169429453 W |
| `P_propulsion = P_useful/eta` | 76.91046324557467580780824 W |
| `P_elec = P_propulsion + P_aux` | 84.91046324557467580780824 W |

## Climb, 14 m/s at a 10° flight path

`n = 1`, `gamma = 10°`, so `L = W cos(gamma)` and `T = D + W sin(gamma)`.

| Quantity | Value |
|---|---|
| `q` | 120.05 Pa |
| `CL` | 0.5027938854163458020298246 |
| `CD` | 0.04683371728776211099151298 |
| `D` | 2.248955104158336569812454 N |
| `T = D + W sin(gamma)` | 6.506222357951842833729171 N |
| `P_useful` | 91.08711301132579967220839 W |
| `P_elec` | 173.6129327478650903131062 W |
| ground speed in still air, `V cos(gamma)` | 13.7873085421709128311344 m/s |

The climb needs 2.05 times the cruise's electrical power at a *lower* airspeed,
which is the point of separating the two: a chain efficiency and a cruise draw
say nothing about what a climb costs.

## The pack

5000 mAh at 14.8 V nominal, 80% usable, with a 20% mission reserve.

| Quantity | Value |
|---|---|
| capacity | 5 Ah = 18000 C |
| `E_nominal = Q V` | 266400 J = 74 Wh |
| `E_usable = E_nominal × 0.8` | 213120 J = 59.2 Wh |
| `E_budget = E_usable × (1 − 0.2)` | 170496 J = 47.36 Wh |
| constant-draw endurance at the cruise draw, `E_budget/P_elec` | 2007.950416038812879198584 s = 33.46584026731354798664307 min |

The reserve is applied once, to the usable energy, and never inside a segment.
The usable fraction and the reserve are separate: the first is a property of the
pack, the second a decision about the flight.

## A cruise segment with a headwind

600 s at the cruise condition above, with a 3 m/s headwind along the track
(`wind_along_track = −3 m/s`).

| Quantity | Value |
|---|---|
| `V_ground = V cos(gamma) + w` | 13 m/s |
| `d = V_ground t` | 7800 m |
| `E = P_elec t` | 50946.27794734480548468494 J |

The same 600 s segment in still air covers 9600 m, and with a 3 m/s tailwind
11400 m. The energy is identical in all three: the aircraft flies the same
airspeed for the same time and only the ground track changes. That is why a
return leg is stated as its own segment with its own wind rather than inferred
from an outbound one.

## A six-leg mission with an autopilot aboard

The legs the browser tests fly, in order, against the pack above with its 20%
reserve. The 8 W of auxiliary draw is split across the avionics that actually
draw it — 5 W of autopilot and 3 W of receiver and servos — and is added to
every leg exactly once, including the two whose propulsion figure was entered
rather than computed: an entered estimate is the *propulsion* draw, so switching
a leg between the two models cannot silently change whether the avionics were
counted.

| Leg | Model | Condition | Time | Electrical power | Ground distance | Energy |
|---|---|---|---|---|---|---|
| launch | entered 250 W | 12 m/s | 12 s | 258 W | 144 m | 3096 J |
| climb | polar | 14 m/s at 10° | 60 s | 173.6129327478650903 W | 827.2385125302547699 m | 10416.77596487190542 J |
| cruise | polar | 16 m/s level | 600 s | 84.91046324557467581 W | 9600 m | 50946.27794734480548 J |
| loiter | polar | 16 m/s level | 300 s | 84.91046324557467581 W | 4800 m | 25473.13897367240274 J |
| return | polar | 16 m/s level, 3 m/s headwind | 600 s | 84.91046324557467581 W | 7800 m | 50946.27794734480548 J |
| recovery | entered 20 W | 10 m/s | 30 s | 28 W | 300 m | 840 J |

| Total | Value |
|---|---|
| duration | 1602 s |
| ground distance | 23471.23851253025477 m |
| energy required | 141718.4708332339191 J |
| energy budget (from above) | 170496 J |
| margin | 28777.5291667660809 J |

The largest continuous demand is the launch's 258 W, which is an entered figure:
the worst moment of this mission is on a leg no model computed. The reserve is
held back once, against the total, and never inside a leg.

## What these fixtures are not

They exercise the implemented relations against independently computed values.
They are not a validation of any aircraft: the polar coefficients, the chain
efficiency and the usable fraction are all synthetic assumptions, and a real
design's numbers come from bench tests and flight logs with their own evidence
grades.
