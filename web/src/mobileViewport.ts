const FOCUS_SCROLL_PADDING = 12;

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

  const down = rect.bottom - bottomLimit;
  const up = topLimit - rect.top;
  const delta = down > 0 ? down : -up;
  const container = findScrollableParent(target);
  if (container) {
    container.scrollBy({ top: delta, behavior: 'smooth' });
    return;
  }
  window.scrollBy({ top: delta, behavior: 'smooth' });
}

export function setupMobileViewportManager(): () => void {
  if (typeof window === 'undefined' || typeof document === 'undefined') return () => {};

  const root = document.documentElement;
  const body = document.body;
  let raf = 0;

  const sync = () => {
    raf = 0;
    const vv = window.visualViewport;
    const visualHeight = Math.round(vv?.height ?? window.innerHeight);
    const visualTop = Math.round(vv?.offsetTop ?? 0);
    const appHeight = Math.max(1, visualHeight);
    const keyboardOffset = Math.max(0, Math.round(window.innerHeight - visualHeight - visualTop));

    root.style.setProperty('--visual-viewport-height', `${visualHeight}px`);
    root.style.setProperty('--app-height', `${appHeight}px`);
    root.style.setProperty('--keyboard-offset', `${keyboardOffset}px`);

    const keyboardOpen = keyboardOffset > 0;
    root.classList.toggle('keyboard-open', keyboardOpen);
    body.classList.toggle('keyboard-open', keyboardOpen);
  };

  const scheduleSync = () => {
    if (raf) return;
    raf = window.requestAnimationFrame(sync);
  };

  const onFocusIn = (ev: FocusEvent) => {
    if (!isEditableElement(ev.target)) return;
    scheduleSync();
    window.setTimeout(() => ensureFocusedElementVisible(ev.target), 50);
    window.setTimeout(() => ensureFocusedElementVisible(ev.target), 250);
  };

  const onFocusOut = () => {
    window.setTimeout(scheduleSync, 50);
    window.setTimeout(scheduleSync, 260);
  };

  scheduleSync();
  window.addEventListener('resize', scheduleSync, { passive: true });
  window.addEventListener('orientationchange', scheduleSync, { passive: true });
  window.addEventListener('focusin', onFocusIn, true);
  window.addEventListener('focusout', onFocusOut, true);
  window.visualViewport?.addEventListener('resize', scheduleSync, { passive: true });
  window.visualViewport?.addEventListener('scroll', scheduleSync, { passive: true });

  return () => {
    window.removeEventListener('resize', scheduleSync);
    window.removeEventListener('orientationchange', scheduleSync);
    window.removeEventListener('focusin', onFocusIn, true);
    window.removeEventListener('focusout', onFocusOut, true);
    window.visualViewport?.removeEventListener('resize', scheduleSync);
    window.visualViewport?.removeEventListener('scroll', scheduleSync);
    if (raf) window.cancelAnimationFrame(raf);
  };
}
