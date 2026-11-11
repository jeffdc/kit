// One icon set: 24px grid, 1.75 stroke, square caps and mitred joins to match
// the flat, square world. Each returns an inline SVG string.

const svg = (body, cls = "") =>
  `<svg class="ic ${cls}" viewBox="0 0 24 24" width="24" height="24" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="square" stroke-linejoin="miter">${body}</svg>`;

export const search = () => svg('<circle cx="10.5" cy="10.5" r="6.5"/><path d="M15.5 15.5 21 21"/>');
export const close = () => svg('<path d="M5 5l14 14M19 5 5 19"/>');
export const plus = () => svg('<path d="M12 4v16M4 12h16"/>');
export const back = () => svg('<path d="M15 4 7 12l8 8"/>');
export const external = () => svg('<path d="M14 4h6v6M20 4l-9 9M18 14v6H4V6h6"/>');
export const trash = () => svg('<path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13"/>');
export const pencil = () => svg('<path d="M4 20h4L19 9l-4-4L4 16zM13 7l4 4"/>');
export const settings = () => svg('<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><rect x="14" y="5" width="4" height="4"/><rect x="6" y="15" width="4" height="4"/>');
export const paste = () => svg('<path d="M9 4h6v3H9zM9 5.5H5V21h14V5.5h-4"/>');
export const check = () => svg('<path d="M4 12.5 9.5 18 20 6"/>');

// Tab icons: a wishlist bookmark, an open book, a stack of finished books.
export const tabWishlist = () => svg('<path d="M6 3h12v18l-6-4.5L6 21z"/>');
export const tabReading = () => svg('<path d="M3 5c3-1 6-1 9 1.5C15 4 18 4 21 5v14c-3-1-6-1-9 1.5C9 18 6 18 3 19zM12 6.5v14"/>');
export const tabHistory = () => svg('<path d="M4 20h16M5 16h14M6 12h12M7 8h10"/>');

export function star(filled) {
  return `<svg class="star${filled ? " on" : ""}" viewBox="0 0 24 24" width="24" height="24" aria-hidden="true" focusable="false"><path d="M12 2.8l2.75 5.86 6.42.75-4.74 4.41 1.27 6.34L12 16.98 6.3 20.16l1.27-6.34L2.83 9.41l6.42-.75z" fill="${filled ? "currentColor" : "none"}" stroke="currentColor" stroke-width="1.5" stroke-linejoin="miter"/></svg>`;
}
