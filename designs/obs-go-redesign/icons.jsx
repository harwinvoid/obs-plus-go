// Line icons, 24px grid, 1.6 stroke.
const ICON_PATHS = {
  cloud: <path d="M7 18h9.2a3.6 3.6 0 0 0 .4-7.18A5.1 5.1 0 0 0 7.35 9.3 4.4 4.4 0 0 0 7 18Z" />,
  refresh: (
    <>
      <path d="M20 11a8 8 0 1 1-2.35-5.65" />
      <path d="M20 4.5V11h-6.5" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" />
    </>
  ),
  upload: (
    <>
      <path d="M12 15.5V4.5" />
      <path d="m7.5 9 4.5-4.5L16.5 9" />
      <path d="M4.5 16.5v2a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2v-2" />
    </>
  ),
  download: (
    <>
      <path d="M12 4.5v11" />
      <path d="m7.5 11 4.5 4.5 4.5-4.5" />
      <path d="M4.5 16.5v2a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2v-2" />
    </>
  ),
  folder: <path d="M3.5 7.5A2.5 2.5 0 0 1 6 5h3.6l1.9 2H18a2.5 2.5 0 0 1 2.5 2.5v7A2.5 2.5 0 0 1 18 19H6a2.5 2.5 0 0 1-2.5-2.5z" />,
  file: (
    <>
      <path d="M6.5 3.5h7l4.5 4.5v12.5h-11.5z" />
      <path d="M13.5 3.5V8H18" />
    </>
  ),
  plus: (
    <>
      <path d="M12 5v14" />
      <path d="M5 12h14" />
    </>
  ),
  trash: (
    <>
      <path d="M4.5 7h15" />
      <path d="M9 7V5.5A1.5 1.5 0 0 1 10.5 4h3A1.5 1.5 0 0 1 15 5.5V7" />
      <path d="M6.5 7l.7 11.2A2 2 0 0 0 9.2 20h5.6a2 2 0 0 0 2-1.8L17.5 7" />
    </>
  ),
  chevron: <path d="m9.5 6 6 6-6 6" />,
  check: <path d="m5 12.5 4.5 4.5L19 7" />,
  x: (
    <>
      <path d="m6 6 12 12" />
      <path d="m18 6-12 12" />
    </>
  ),
  more: (
    <>
      <circle cx="6" cy="12" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1.2" fill="currentColor" stroke="none" />
      <circle cx="18" cy="12" r="1.2" fill="currentColor" stroke="none" />
    </>
  ),
  link: (
    <>
      <path d="M10 13.5a4 4 0 0 0 5.7 0l2.8-2.8a4 4 0 0 0-5.7-5.7L11.5 6.3" />
      <path d="M14 10.5a4 4 0 0 0-5.7 0l-2.8 2.8a4 4 0 0 0 5.7 5.7l1.3-1.3" />
    </>
  ),
  bucket: (
    <>
      <ellipse cx="12" cy="6" rx="7.5" ry="2.8" />
      <path d="M4.5 6v12c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8V6" />
      <path d="M4.5 12c0 1.55 3.36 2.8 7.5 2.8s7.5-1.25 7.5-2.8" />
    </>
  ),
  square: <rect x="4.5" y="4.5" width="15" height="15" rx="2.5" />,
  settings: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 3.5v2M12 18.5v2M3.5 12h2M18.5 12h2M5.9 5.9l1.5 1.5M16.6 16.6l1.5 1.5M18.1 5.9l-1.5 1.5M7.4 16.6l-1.5 1.5" />
    </>
  ),
  logout: (
    <>
      <path d="M14 5.5H6.5A1.5 1.5 0 0 0 5 7v10a1.5 1.5 0 0 0 1.5 1.5H14" />
      <path d="M17 8.5 20.5 12 17 15.5" />
      <path d="M10 12h10" />
    </>
  ),
};

function Icon({ name, size = 16, strokeWidth = 1.6, className }) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {ICON_PATHS[name] || ICON_PATHS.file}
    </svg>
  );
}

const FILE_TINTS = {
  html: "#b45309",
  json: "#475569",
  txt: "#64748b",
  log: "#64748b",
  ico: "#0e7490",
  png: "#0369a1",
  jpg: "#0369a1",
  webp: "#0369a1",
  svg: "#7c3aed",
  zip: "#6d28d9",
  woff2: "#0e9f8e",
};

function fileTint(ext) {
  return FILE_TINTS[ext] || "#64748b";
}

Object.assign(window, { Icon, fileTint });
