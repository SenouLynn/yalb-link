# Task 13 — Carbon spar fit and wing stiffness

**Outcome:** whether a chosen spar physically fits inside a wing at every station,
and how stiff that wing is with it. Dependencies:
[03](03-geometry-and-configurations.md), [12](12-airfoil-sections.md).

[Task 11](11-external-handoff.md) reserves structural load envelope and wing
stiffness/bending as additional engineering work "unless separately implemented".
This is that separate implementation, and it is deliberately narrow: **fit and
stiffness only.** No strength result of any kind follows from it.

## Scope boundary, stated first

Supported: the geometric fit of a spar inside the section envelope under taper and
twist; second moment of area for the implemented cross sections; a spanwise load
distribution; shear, bending moment, flexural rigidity and deflection; and the
inversion that returns the rigidity needed for a target tip deflection.

Not supported, and reported as unsupported rather than omitted: allowable stress,
margin of safety, failure, local or global buckling, crushing, shear web and bond
line, joiner and telescoped-joint strength, fatigue, torsional stiffness,
aeroelastic twist, divergence and flutter. A deflection number printed beside a
spar diameter reads as an endorsement unless something explicitly says it is not
one, so the refusal is a code path, not a sentence in a document.

## Work

- Follow the [handoff's source rules](README.md). The book has no structures
  chapter, so **nothing in this task may claim `SourceBook`**. Beam relations are
  standard Euler–Bernoulli; the load distribution cites
  [Schrenk, NACA TM-948](https://ntrs.nasa.gov/citations/19930094469).
- Add the dimensions this needs. Keep an elastic modulus distinct from a wing
  loading and a bending moment distinct from an energy, even though each pair
  shares an SI unit. Refusing to interchange numerically identical but
  semantically different quantities is the existing thesis of the units layer.
- **Never default a carbon modulus.** A unidirectional pultruded rod, a
  roll-wrapped tube with off-axis plies and a hand layup differ by a large factor
  in axial modulus. Require supplied evidence exactly as a `CLmax` already requires
  a basis and a scope, and record the material form so that a modulus inconsistent
  with the form is visible to a reader.
- Support the cross sections RC wings actually use: solid rod, tube, rectangular
  strip, and a **spar cap pair**. The cap pair matters because its second moment is
  dominated by the parallel-axis term; modelling two caps as one rectangle
  understates rigidity by an order of magnitude.
- Check fit against the lofted section from Task 12, in the spar's own frame. A
  spar is a straight member through twisted ribs: the ribs rotate, the spar does
  not. Report where along the span a constant-section spar stops fitting, and
  support stepped or telescoped segments as the answer to that.
- Compute stiffness against a stated flight case, reusing the existing weight and
  required-lift relations for total load and the case's load factor.
- Report spar mass and its effect on all-up mass. Do not iterate silently: mass
  feeding load feeding required rigidity is physical design iteration, which the
  handoff requires be resolved by user revision or by a bounded solver with
  explicit convergence and failure results.

## Equation contracts

Second moment of area about the bending axis:

```text
rod        I = pi d^4 / 64
tube       I = pi (d_o^4 - d_i^4) / 64
rectangle  I = b h^3 / 12                      h is the bending direction
cap pair   I = 2 [ b t^3 / 12 + b t (h/2)^2 ]  h is the cap separation
```

The cap pair assumes the caps act as a unit, which requires an adequate shear web
and bond. **No web is modelled and no bond is checked.**

Spanwise load, over a semi-span `s = b/2`, normalised so the distribution
integrates to the total load:

```text
L_total   = n m g
uniform            L'(y) = L_total / b
chord-proportional L'(y) proportional to c(y)
elliptical         L'(y) proportional to sqrt(1 - (y/s)^2)
Schrenk            L'(y) = ( L'_elliptical(y) + L'_chord(y) ) / 2
```

Schrenk's approximation is the recommended selection and never a silent one.
**It ignores twist.** A washed-out wing carries less tip load than Schrenk
predicts, so the root bending moment is conservative — safe for sizing, but it
means the twist that changes the spar *fit* below does not change the spar *load*
here. Also ignored: sweep, fuselage carry-through and aeroelastic redistribution.

Beam relations, integrated from the tip inward and deflection from the root out:

```text
V(y) = integral from y to s of L'(eta) d_eta
M(y) = integral from y to s of V(eta) d_eta
kappa(y) = M(y) / (E I(y))
w(y) from integrating kappa twice with w(0) = 0 and w'(0) = 0
EI_required = the rigidity giving a stated tip deflection, by inversion
```

The root condition is a rigid encastre. **A real wing joiner or centre section is
not one**, and a two-panel wing on a tube joiner is softer at the root than this
model. State the integration scheme and station count; both are inputs.

Fit at a station, with the spar at a stated chord fraction and a stated clearance:

```text
c(y)       from the solved planform
section    lofted per Task 12, scaled by c(y)
theta(y)   local twist about the stated twist axis
frame      rotate the scaled section by -theta(y): the spar does not twist
available  room at the spar's position in that rotated frame, less 2 * clearance
required   the spar's own envelope
margin     available - required
```

The clearance test depends on the shape and is recorded with the result. A
rectangular cap or strip is checked on vertical extent across its chordwise
footprint, taking the minimum. **A round rod or tube is checked on the largest
inscribed circle, not the vertical gap**: a circle of diameter `d` needs `d` of
vertical room across `d` of chordwise width, and the section is narrowing over that
width, so a vertical-gap test passes rods that do not fit.

## Acceptance checks

- A uniform running load `w` on a constant-rigidity cantilever of length `L`
  reproduces the closed forms: tip deflection `w L^4 / (8 EI)` and root moment
  `w L^2 / 2`. This is what establishes the integrator, not a self-consistency check.
- An elliptical distribution integrates back to `n m g` over the span, and its
  constant-rigidity deflection matches its closed form.
- Halving the integration step changes tip deflection by less than the stated
  tolerance. The station count is reported with the result.
- The Schrenk distribution lies between its two constituents everywhere and
  integrates to the same total. Its assumptions, including that twist is ignored,
  are attached to the equation and appear in the result.
- Second moments: the tube relation approaches the rod relation as wall thickness
  approaches the radius; the cap pair with zero separation approaches two stacked
  rectangles, and its parallel-axis term dominates at realistic separations.
- A material with no evidence is a **missing field**, and no code path produces a
  modulus without one. Verified by mutation: removing the check must fail the test.
- A material form is recorded and travels with the result, so a layup modulus and a
  pultrusion modulus are distinguishable after the fact.
- An untapered, untwisted wing with a single section fits identically at every
  station. A tapered wing with a constant-section spar reports the station where it
  first interferes, and that station moves outboard as taper ratio approaches 1.
- **Twist changes the fit.** The same wing at 0 and -5 degrees twist gives different
  available depth at the same station, and the difference grows with distance from
  the twist axis. A wing whose spar clears when flat may foul once washed out.
- A round spar that passes a vertical-gap check but fails the inscribed-circle
  check is reported as interfering, and the result names which test was applied.
- Stepped segments must be contiguous and each must fit over its own range. A
  joint overlap is recorded; **no joint strength is assessed or implied.**
- Spar mass is reported with the change it makes to all-up mass. Nothing
  re-solves the load automatically.
- The strength refusal always returns unsupported, for every configuration and
  every valid spar, in the same way a geometry-only airframe refuses a handling
  result.
- Requesting a rigidity for a target tip deflection returns a value that, fed back
  through the forward path, reproduces that deflection within tolerance.
- Provenance: no equation added by this task carries `SourceBook`.
- Exported parameters carry their unit, role and the equation revision behind each
  derived value, and the dependency graph including the new keys stays acyclic.

**Not established by this task:** that a wing is strong enough, safe, airworthy, or
able to survive a landing, a launch, a gust or an aerobatic entry. It reports how
much a modelled beam bends under a modelled load. Correct arithmetic here is not a
validated structure.
