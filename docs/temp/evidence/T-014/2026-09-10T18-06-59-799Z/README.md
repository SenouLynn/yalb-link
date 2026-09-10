# Mock UI review

Date: 2026-09-10T18:07:24.610Z
Base revision: cc3dff3a9823e76c0f10b602ee9061533c4ff4fd
Frontend diff SHA256: 87df28042df185c6b47ad30403a31d1804d85c53d87cc1d780a5be13ee52a3e2
URL: http://127.0.0.1:3001/?source=mock

Command: node scripts/shoot.mjs (host Vite on 3001). Chrome CDP with software WebGL, fresh profile per viewport, real wall-clock settling.

Expected and observed: two streaming vehicles, LOST observer, auto-loaded missions, readable provenance, instrument tier above developer tier, no page overflow. Panel hide/restore, mission refresh twice, vehicle switching and keyboard tabs passed at both widths. Application runtime exceptions/errors: none. Public basemap request errors, if any, are recorded separately in observations.json.

PNG captures and matching text dumps include fleet, vehicle and stacked instruments; JSON records computed layout and console events. Basemap loading depends on public internet; these synthetic fixtures do not establish live telemetry behavior.
