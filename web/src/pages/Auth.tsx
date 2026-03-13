import { useState, useCallback } from 'react';
import { useAuthStore } from '../store';

type Step = 'identifier' | 'code';

export default function Auth() {
  const [step, setStep] = useState<Step>('identifier');
  const [identifier, setIdentifier] = useState('');
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const [logoFailed, setLogoFailed] = useState(false);
  const { requestCode, verifyCode } = useAuthStore();

  const handleRequestCode = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const autoLoggedIn = await requestCode(identifier);
      if (!autoLoggedIn) setStep('code');
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Произошла ошибка');
    } finally {
      setLoading(false);
    }
  }, [identifier, requestCode]);

  const handleVerifyCode = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await verifyCode(identifier, code);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Неверный или истекший код');
    } finally {
      setLoading(false);
    }
  }, [identifier, code, verifyCode]);

  return (
    <div className="min-h-[var(--app-height)] min-w-0 max-w-[100vw] w-full flex items-center justify-center bg-surface dark:bg-dark-bg p-3 sm:p-4 safe-area-padding overflow-x-hidden">
      <div className="w-full max-w-[380px] min-w-0 overflow-x-hidden">
        <div className="text-center mb-6 sm:mb-8 min-w-0">
          <div className="flex items-center justify-center mb-4">
            {logoFailed ? (
              <div className="w-20 h-20 rounded-[20px] bg-primary text-white flex items-center justify-center text-[26px] font-bold shadow-lg shadow-black/20">
                P
              </div>
            ) : (
              <img
                src="/brand/pulse.png"
                alt="pulse"
                className="w-20 h-20 rounded-[20px] shadow-lg shadow-black/20"
                onError={() => setLogoFailed(true)}
              />
            )}
          </div>
          <h1 className="text-[24px] font-bold text-txt dark:text-[#e7e9ea] tracking-[-0.3px]">pulse</h1>
          <p className="text-txt-secondary dark:text-[#8b98a5] text-[14px] mt-1">Портал компании</p>
        </div>

        <div className="bg-white dark:bg-dark-elevated rounded-compass shadow-compass-dialog overflow-hidden overflow-x-hidden border border-transparent dark:border-dark-border min-w-0">
          <div className="p-4 sm:p-5 space-y-4 min-w-0">
            {error && (
              <div className="bg-danger/8 text-danger text-[13px] rounded-compass px-3.5 py-2.5 font-medium">{error}</div>
            )}

            {step === 'identifier' ? (
              <form onSubmit={handleRequestCode} className="space-y-4">
                <div>
                  <label className="block text-[13px] font-medium text-txt-secondary dark:text-[#8b98a5] mb-1.5">
                    Email или ключ входа
                  </label>
                  <input
                    type="text"
                    value={identifier}
                    onChange={(e) => setIdentifier(e.target.value)}
                    className="compass-input"
                    placeholder="name@company.com или ключ входа"
                    autoComplete="username"
                    required
                  />
                  <p className="mt-1.5 text-[12px] leading-5 text-txt-secondary dark:text-[#8b98a5]">
                    На email придет одноразовый код для входа. Если введен ключ входа, вход выполнится сразу.
                  </p>
                </div>
                <button type="submit" disabled={loading} className="compass-btn-primary w-full py-3 mt-1">
                  {loading ? (
                    <span className="inline-flex items-center gap-2">
                      <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                      Отправка...
                    </span>
                  ) : (
                    'Получить код'
                  )}
                </button>
              </form>
            ) : (
              <form onSubmit={handleVerifyCode} className="space-y-4">
                <p className="text-[13px] text-txt-secondary dark:text-[#8b98a5]">
                  Код отправлен на <strong className="text-txt dark:text-[#e7e9ea]">{identifier}</strong>
                </p>
                <div>
                  <label className="block text-[13px] font-medium text-txt-secondary dark:text-[#8b98a5] mb-1.5">Код из письма</label>
                  <input
                    type="text"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    className="compass-input text-center tracking-[0.3em] font-mono text-lg"
                    placeholder="000000"
                    maxLength={6}
                    required
                  />
                </div>
                <button type="submit" disabled={loading || code.length < 4} className="compass-btn-primary w-full py-3 mt-1">
                  {loading ? (
                    <span className="inline-flex items-center gap-2">
                      <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24"><circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" fill="none"/><path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                      Вход...
                    </span>
                  ) : (
                    'Войти'
                  )}
                </button>
                <button
                  type="button"
                  onClick={() => { setStep('identifier'); setCode(''); setError(''); }}
                  className="w-full text-[13px] text-primary hover:underline"
                >
                  Указать другой email/ключ
                </button>
              </form>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
