import unittest
import math
from types import SimpleNamespace
from unittest.mock import Mock
from sitl_motion import bounded_wait, distance, offset, HOME, Experiment, Relay, route_points


class MotionTests(unittest.TestCase):
    def test_route_scale_and_direction(self):
        north, east = offset(100, 0), offset(100, 100)
        self.assertGreater(north[0], HOME[0])
        self.assertEqual(north[1], HOME[1])
        self.assertGreater(east[1], north[1])
        self.assertAlmostEqual(distance(HOME, north), 100, places=4)
        self.assertAlmostEqual(distance(north, east), 100, places=2)

    def test_figure_eight_has_two_closed_circuits_and_opposing_lobes(self):
        points = [(0, 0), *route_points('figure-eight')]
        for index in [8, 16, 24, 32]:
            self.assertLess(math.hypot(*points[index]), 1e-10)
        self.assertEqual(len(points), 33)
        self.assertGreater(max(n for n, e in points), 39)
        self.assertLess(min(n for n, e in points), -39)
        # Signed turning direction reverses between the two lobes.
        def turn(a, b, c):
            return (b[0]-a[0])*(c[1]-b[1])-(b[1]-a[1])*(c[0]-b[0])
        self.assertLess(turn(*points[1:4]) * turn(*points[9:12]), 0)
        for a, b in zip(points[1:17], points[17:]):
            self.assertAlmostEqual(a[0], b[0])
            self.assertAlmostEqual(a[1], b[1])

    def test_snake_progresses_with_five_alternating_extrema_and_finite_end(self):
        points = route_points('snake')
        self.assertEqual([round(e) for n, e in points[::2]], [20, -20, 20, -20, 20])
        self.assertTrue(all(b[0] > a[0] for a, b in zip(points, points[1:])))
        self.assertEqual(points[-1][0], 150)
        self.assertAlmostEqual(points[-1][1], 0)
        with self.assertRaises(ValueError):
            route_points('unbounded')

    def test_silence_times_out(self):
        ticks = iter([0, 0, 1, 2, 3])
        with self.assertRaisesRegex(TimeoutError, 'no telemetry'):
            bounded_wait(lambda: None, lambda _: True, 3, 'no telemetry', lambda: next(ticks))

    def test_unrelated_messages_cannot_satisfy_wait(self):
        messages = iter(['attitude', None, 'position'])
        ticks = iter([0, 1, 2, 3])
        self.assertEqual(bounded_wait(lambda: next(messages), lambda m: m == 'position',
                                    4, 'position', lambda: next(ticks)), 'position')

    def test_receive_failure_propagates(self):
        def disconnected():
            raise ConnectionError('disconnected')
        with self.assertRaises(ConnectionError):
            bounded_wait(disconnected, lambda _: True, 1, 'position')

    def test_position_transition_precedes_speed_request(self):
        exp = object.__new__(Experiment)
        calls=[]
        exp.commands=object(); exp.events=object()
        exp.record=lambda *a, **kw: None
        exp.link=SimpleNamespace(mav=SimpleNamespace(set_position_target_global_int_send=lambda *a: calls.append('target')))
        exp.mav=SimpleNamespace(MAV_FRAME_GLOBAL_RELATIVE_ALT_INT=6, MAV_CMD_DO_CHANGE_SPEED=178)
        exp.command=lambda *a: calls.append('speed')
        lat,lon=offset(100,0)
        endpoint=SimpleNamespace(lat=round(lat*1e7),lon=round(lon*1e7),relative_alt=20000)
        exp.wait=lambda *a: endpoint
        exp.target(100,0)
        self.assertEqual(calls,['target','speed'])

    def test_rejected_command_fails(self):
        exp=object.__new__(Experiment)
        exp.commands=object();exp.record=lambda *a,**kw:None
        exp.system=exp.component=1
        exp.link=SimpleNamespace(mav=SimpleNamespace(command_long_send=Mock()))
        exp.mav=SimpleNamespace(MAV_RESULT_ACCEPTED=0)
        exp.wait=lambda *a:SimpleNamespace(result=2)
        with self.assertRaisesRegex(RuntimeError,'rejected'):
            exp.command(400,1)

    def test_relay_requires_simulator_peer_and_propagates_failure(self):
        relay=object.__new__(Relay)
        relay.error=None;relay.peer=None;relay.vehicle=Mock()
        with self.assertRaisesRegex(RuntimeError,'SITL must stream'):
            relay.send(b'command')
        relay.error=OSError('broken socket')
        with self.assertRaisesRegex(RuntimeError,'relay failed'):
            relay.send(b'command')
        relay.vehicle.sendto.assert_not_called()


if __name__ == '__main__':
    unittest.main()
