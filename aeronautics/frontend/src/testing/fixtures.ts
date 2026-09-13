import type { Evaluation, PowerSearchResponse } from '../api/contract.ts'

// Shared pieces of a stand-in evaluation.
//
// These tests are about request ordering and about what the page does with an
// answer, not about the physics in one, so their evaluations are written by
// hand. The power half is the same in all of them — a design that states no
// propulsion at all — so it lives here rather than being retyped, and it is
// written out in full rather than cast, so that adding a field to the contract
// fails these fixtures instead of silently leaving them describing something
// the service no longer sends.

/** unpowered is the power half of an evaluation for a design with no propulsion. */
export function unpowered(): Pick<
  Evaluation,
  'powerFeasibility' | 'thrustChecks' | 'electrical' | 'mission'
> {
  return {
    powerFeasibility: { status: 'missing', checks: [], continuousDemand: null, peakDemand: null },
    thrustChecks: [],
    electrical: { status: 'missing', loads: [], continuous: null, peak: null, complete: false },
    mission: {
      status: 'missing',
      energyStatus: 'unknown',
      segments: [],
      traces: [],
      requiredEnergy: null,
      usableEnergy: null,
      budget: null,
      totalDuration: null,
      totalDistance: null,
      peakContinuousPower: null,
      reserveFraction: 0,
      complete: false,
    },
  }
}

/**
 * noPowerSearch is a transport's power search for a test that never asks for
 * one. It rejects rather than answering, because a stub that quietly returned
 * an empty result would let a test that did ask pass while describing nothing.
 */
export function noPowerSearch(): Promise<PowerSearchResponse> {
  return Promise.reject(new Error('this test asks for no power search'))
}
