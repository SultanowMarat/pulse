const FOCUS_SCROLL_PADDING = 12;
const KEYBOARD_OPEN_THRESHOLD = 80;

function isEditableElement(node: EventTarget | null): node is HTMLElement {
  if (!(node instanceof HTMLElement)) return false;
  if (node.isContentEditable) return true;
  const tag = node.tagName.toLowerCase();
  if (tag === 'textarea') return true;
  if (tag === 'input') {
    const input = node as HTMLInputElement;
    return input.type !== 'button' && input.type !== 'checkbox' && input.type !== 'radio' && input.type !== 'file';
  }
  return false;
}

function isScrollableY(el: HTMLElement): boolean {
  const style = window.getComputedStyle(el);
  const overflowY = style.overflowY;
  if (overflowY !== 'auto' && overflowY !== 'scroll') return false;
  return el.scrollHeight > el.clientHeight + 1;
}

function findScrollableParent(from: HTMLElement): HTMLElement | null {
  let cur: HTMLElement | null = from.parentElement;
  while (cur && cur !== document.body) {
    if (isScrollableY(cur)) return cur;
    cur = cur.parentElement;
  }
  return null;
}

function findAnyScrollableParent(from: HTMLElement | null): HTMLElement | null {
  let cur: HTMLElement | null = from;
  while (cur && cur !== document.body) {
    if (isScrollableY(cur)) return cur;
    cur = cur.parentElement;
  }
  return null;
}

function ensureFocusedElementVisible(target: HTMLElement) {
  if (!target.isConnected) return;
  const vv = window.visualViewport;
  const viewportHeight = vv?.height ?? window.innerHeight;
  const viewportTop = vv?.offsetTop ?? 0;
  const viewportBottom = viewportTop + viewportHeight;
  const rect = target.getBoundingClientRect();
  const topLimit = viewportTop + FOCUS_SCROLL_PADDING;
  const bottomLimit = viewportBottom - FOCUS_SCROLL_PADDING;

  if (rect.top >= topLimit && rect.bottom <= bottomLimit) return;

  const container = findScrollableParent(target);
  if (!container) return;

  const containerRect = container.getBoundingClientRect();
  const down = rect.bottom - Math.min(bottomLimit, containerRect.bottom - FOCUS_SCROLL_PADDING);
  const up = Math.max(topLimit, containerRect.top + FOCUS_SCROLL_PADDING) - rect.top;
  const delta = down > 0 ? down : -up;
  if (Math.abs(delta) < 1) return;

  container.scrollTo({
    top: Math.max(0, container.scrollTop + delta),
    behavior: 'auto',
  });
}

export function setupMobileViewportManager(): () => void {
  if (typeof window === 'undefined' || typeof document === 'undefined') return () => {};

  const root = document.documentElement;
  const body = document.body;
  let raf = 0;
  let baseInnerHeight = Math.max(window.innerHeight, Math.round(window.visualViewport?.height ?? 0));
  let hasFocusedEditable = false;
  let keyboardOpen = false;
  let touchStartY = 0;

  const sync = () => {
    raf = 0;
    hasFocusedEditable = isEditableElement(document.activeElement);
    const vv = window.visualViewport;
    const visualHeight = Math.round(vv?.height ?? window.innerHeight);
    const visualWidth = Math.round(vv?.width ?? window.innerWidth);
    const visualTop = Math.round(vv?.offsetTop ?? 0);
    const appHeight = Math.max(1, visualHeight);
    const keyboardOffset = Math.max(0, Math.round(baseInnerHeight - (visualHeight + visualTop)));
    keyboardOpen = hasFocusedEditable && keyboardOffset > KEYBOARD_OPEN_THRESHOLD;

    root.style.setProperty('--visual-viewport-height', `${visualHeight}px`);
    root.style.setProperty('--visual-viewport-width', `${visualWidth}px`);
    root.style.setProperty('--app-height', `${appHeight}px`);
    root.style.setProperty('--keyboard-offset', `${keyboardOffset}px`);
    root.style.setProperty('--composer-padding-bottom', keyboardOpen ? '2px' : 'max(2px, env(safe-area-inset-bottom))');
    root.classList.toggle('keyboard-open', keyboardOpen);
    body.classList.toggle('keyboard-open', keyboardOpen);

    // iOS/Android can keep body scrolled after focus transition; keep page root pinned.
    if (keyboardOpen && (window.scrollX !== 0 || window.scrollY !== 0)) {
      window.scrollTo({ left: 0, top: 0, behavior: 'auto' });
    }
  };

  const scheduleSync = () => {
    if (raf) return;
    raf = window.requestAnimationFrame(sync);
  };

  const onOrientationChange = () => {
    // Rebaseline after rotation to keep keyboard detection stable.
    window.setTimeout(() => {
      baseInnerHeight = Math.max(window.innerHeight, Math.round(window.visualViewport?.height ?? 0));
      scheduleSync();
    }, 120);
  };

  const onFocusIn = (ev: FocusEvent) => {
    if (!isEditableElement(ev.target)) return;
    hasFocusedEditable = true;
    scheduleSync();
    window.setTimeout(scheduleSync, 120);
    window.setTimeout(scheduleSync, 320);
    window.setTimeout(() => ensureFocusedElementVisible(ev.target), 50);
    window.setTimeout(() => ensureFocusedElementVisible(ev.target), 250);
  };

  const onFocusOut = () => {
    hasFocusedEditable = isEditableElement(document.activeElement);
    window.setTimeout(scheduleSync, 50);
    window.setTimeout(scheduleSync, 260);
  };

  const onTouchStart = (ev: TouchEvent) => {
    if (!keyboardOpen) return;
    if (ev.touches.length > 0) touchStartY = ev.touches[0].clientY;
  };

  const onTouchMove = (ev: TouchEvent) => {
    if (!keyboardOpen) return;
    const target = ev.target as HTMLElement | null;
    const scrollParent = findAnyScrollableParent(target);
    if (!scrollParent) {
      ev.preventDefault();
      return;
    }
    if (ev.touches.length === 0) return;
    const currentY = ev.touches[0].clientY;
    const deltaY = currentY - touchStartY;
    const atTop = scrollParent.scrollTop <= 0;
    const atBottom = scrollParent.scrollTop + scrollParent.clientHeight >= scrollParent.scrollHeight - 1;
    const pullingDownAtTop = atTop && deltaY > 0;
    const pushingUpAtBottom = atBottom && deltaY < 0;
    if (pullingDownAtTop || pushingUpAtBottom) {
      ev.preventDefault();
    }
  };

  scheduleSync();
  window.addEventListener('resize', scheduleSync, { passive: true });
  window.addEventListener('orientationchange', onOrientationChange, { passive: true });
  window.addEventListener('focusin', onFocusIn, true);
  window.addEventListener('focusout', onFocusOut, true);
  window.addEventListener('touchstart', onTouchStart, { passive: true, capture: true });
  window.addEventListener('touchmove', onTouchMove, { passive: false, capture: true });
  window.visualViewport?.addEventListener('resize', scheduleSync, { passive: true });
  window.visualViewport?.addEventListener('scroll', scheduleSync, { passive: true });

  return () => {
    window.removeEventListener('resize', scheduleSync);
    window.removeEventListener('orientationchange', onOrientationChange);
    window.removeEventListener('focusin', onFocusIn, true);
    window.removeEventListener('focusout', onFocusOut, true);
    window.removeEventListener('touchstart', onTouchStart, true);
    window.removeEventListener('touchmove', onTouchMove, true);
    window.visualViewport?.removeEventListener('resize', scheduleSync);
    window.visualViewport?.removeEventListener('scroll', scheduleSync);
    if (raf) window.cancelAnimationFrame(raf);
    root.style.removeProperty('--composer-padding-bottom');
    root.classList.remove('keyboard-open');
    body.classList.remove('keyboard-open');
  };
}
