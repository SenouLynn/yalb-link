# Mock UI review

Date: 2026-09-10T18:47:46.257Z
Base revision: d0e573c0e327114a1ba116e405138063dff20577
Frontend diff SHA256: cc9f575d657414db5cf49638b099636e6d230458d830ebfced1831934be1ff0d
URL: http://127.0.0.1:3001/?source=mock

Command: node scripts/shoot.mjs (host Vite on 3001). Chrome CDP with software WebGL, fresh profile per viewport, real wall-clock settling.

Expected and observed: two streaming vehicles, LOST observer, auto-loaded missions, readable provenance, instrument tier above developer tier, no page overflow. Panel hide/restore, mission refresh twice, vehicle switching and keyboard tabs passed at both widths. Application runtime exceptions/errors: none. Public basemap request errors, if any, are recorded separately in observations.json.

PNG captures and matching text dumps include fleet, vehicle and stacked instruments; JSON records computed layout and console events. Basemap loading depends on public internet; these synthetic fixtures do not establish live telemetry behavior.
