export function installDiagnosisDocumentListeners(options: {
  isModelMenuOpen: () => boolean;
  closeModelMenu: () => void;
  copyCode: (encoded: string) => void;
}) {
  const handlePointerDown = (event: PointerEvent) => {
    const target = event.target;
    if (!(target instanceof Element)) return;
    if (options.isModelMenuOpen() && !target.closest('.diagnosis-model-picker')) {
      options.closeModelMenu();
    }
  };
  const handleKeydown = (event: KeyboardEvent) => {
    if (event.key === 'Escape') options.closeModelMenu();
  };
  const handleMarkdownClick = (event: MouseEvent) => {
    const target = event.target as HTMLElement | null;
    const button = target?.closest<HTMLButtonElement>('[data-code-copy]');
    if (!button) return;
    event.preventDefault();
    options.copyCode(button.dataset.codeCopy ?? '');
  };
  document.addEventListener('pointerdown', handlePointerDown);
  document.addEventListener('keydown', handleKeydown);
  document.addEventListener('click', handleMarkdownClick);
  return () => {
    document.removeEventListener('pointerdown', handlePointerDown);
    document.removeEventListener('keydown', handleKeydown);
    document.removeEventListener('click', handleMarkdownClick);
  };
}

export function copyDiagnosisText(content: string, successMessage: string, onNotice: (message: string) => void) {
  const clipboard = navigator.clipboard;
  if (!clipboard?.writeText) {
    onNotice('当前浏览器不允许直接复制，请手动选择文本。');
    return;
  }
  void clipboard.writeText(content).then(
    () => onNotice(successMessage),
    () => onNotice('当前浏览器不允许直接复制，请手动选择文本。')
  );
}

export function startDiagnosisPanelResize(
  side: 'history' | 'context',
  event: PointerEvent,
  getWidths: () => { history: number; context: number },
  setWidth: (side: 'history' | 'context', value: number) => void
) {
  if (window.innerWidth <= 850) return;
  const startX = event.clientX;
  const startWidth = side === 'history' ? getWidths().history : getWidths().context;
  const move = (moveEvent: PointerEvent) => {
    const delta = moveEvent.clientX - startX;
    const next = side === 'history'
      ? Math.max(180, Math.min(360, startWidth + delta))
      : Math.max(220, Math.min(380, startWidth - delta));
    setWidth(side, next);
  };
  const stop = () => {
    window.removeEventListener('pointermove', move);
    window.removeEventListener('pointerup', stop);
  };
  window.addEventListener('pointermove', move);
  window.addEventListener('pointerup', stop, { once: true });
}
