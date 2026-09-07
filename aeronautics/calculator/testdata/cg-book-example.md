# Fixture — CODE Lab Center of gravity worked example

Source: [Weight and Balance — Center of gravity](https://computationaldesignlab.github.io/aircraft-design/weight_and_balance/cg.html),
CODE Lab Aircraft Design, read 2026-09-06. No upstream commit revision is
published on the page.

The example is a manned twin in US customary units, measured from a nose datum
at `x = 3.4 ft`. Its numbers are used here to check the ported relation. **They
are not RC defaults**, and no mass, arm or role below is a default anywhere in
the calculator.

## What the chapter states

The chapter gives one relation:

```
x_CG = sum(x_CG_k * W_k) / sum(W_k)
```

over component **weights**, and adds that "a similar equation can be used for y
and z axis" while demonstrating only `x`.

**Adaptation.** This package holds masses rather than weights. Under one uniform
standard gravity the `g` cancels between the numerator and the denominator, so
`sum(m_k x_k)/sum(m_k)` is the same station. That is the only adaptation, and it
is recorded on the equation's own `Source.Adaptation` as well as here. All three
axes are evaluated with the relation the chapter sanctions for them.

## Inputs, in the chapter's own units

| Component | Weight (lb) | Moment arm (ft) | Moment (lb·ft) |
|---|---|---|---|
| Wing | 344 | 15.6 | 5366.4 |
| Fuselage | 367 | 15.7 | 5761.9 |
| Horizontal tail | 42 | 32.4 | 1360.8 |
| Vertical tail | 39 | 32.7 | 1275.3 |
| Main landing gear | 78 | 16.1 | 1255.8 |
| Nose landing gear | 26 | 7.4 | 192.4 |
| Propulsion system | 1663 | 12.2 | 20288.6 |
| Miscellaneous | 555 | 15.7 | 8713.5 |
| **Total** | **3114** | — | **44214.7** |

The chapter also gives `MAC = 4.3 ft` with its leading edge at `x = 13.85 ft`.

## Expected values

Computed independently at 50 significant digits from the component table, **not**
read back from the chapter's displayed output and **not** taken from its rounded
moment column. Each product in the moment column is exact to one decimal place
and the independently formed sum agrees with the chapter's total exactly.

| Quantity | Value | In the chapter's units |
|---|---|---|
| `sum(W_k)` | `1412.48664018 kg` | `3114 lb` |
| `sum(x_k W_k)` | `6112.9013312485272 kg·m` | `44214.7 lb·ft` |
| `x_CG` | `4.3277586897880539499036608863198458574181117533718 m` | `14.198683365446371226718047527296082209377007064868 ft` |
| `(x_CG − x_le_MAC)/MAC` | `0.081089154754970052725127331929321444041164433690233` | — |

The unit conversions are exact: `1 lb = 0.45359237 kg` and `1 ft = 0.3048 m`.

## Discrepancies

None. The chapter displays `x = 14.2 ft` and `8% MAC`; both are display rounding
of the values above, not a difference in the relation. `14.198683…` rounds to
`14.2`, and `0.08108…` rounds to `8%`.

## Tolerances

`TestTheBookCentreOfGravityExampleReproduces` compares in SI with an absolute
term of `1e-10` in the value's own SI unit plus a relative term of `1e-12`, the
same tolerance the other design fixtures use. The original-unit comparison
against the chapter's displayed `14.2 ft` uses `0.05 ft`, which is the width of
its own display rounding and nothing tighter.

## What the fixture does not establish

The chapter's `%MAC` figure is a geometric reference. It is not a static margin,
a neutral point or a trim result, and this package implements none of those; the
`mass.station-fraction-of-mac` relation is recorded as **derived**, because the
chapter states no chord-fraction relation of its own.
