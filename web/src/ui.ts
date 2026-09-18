export let language: 'ru' | 'en' = localStorage.getItem('courier_language') === 'en' ? 'en' : 'ru';
document.documentElement.lang = language;
export function setLanguage(value: 'ru' | 'en') { language = value; localStorage.setItem('courier_language', value); document.documentElement.lang = value; }
export function t(ru: string, en: string) { return language === 'ru' ? ru : en; }
export function escape(value: unknown): string { return String(value ?? '').replace(/[&<>"']/g, character => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[character]!)); }
export function input(id: string, label: string, value: unknown = '', type = 'text', extra = '') { return `<label>${escape(label)}<input id="${id}" type="${type}" value="${escape(value)}" ${extra}></label>`; }
export function area(id: string, label: string, value: unknown = '', rows = 5) { return `<label>${escape(label)}<textarea id="${id}" rows="${rows}" spellcheck="false">${escape(value)}</textarea></label>`; }
export function value(id: string): string { return document.querySelector<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>(`#${id}`)!.value; }
export function checked(id: string): boolean { return document.querySelector<HTMLInputElement>(`#${id}`)!.checked; }
export function on(id: string, action: () => unknown) { document.getElementById(id)?.addEventListener('click', () => { Promise.resolve().then(action).catch(showError); }); }
export function showError(error: unknown) {
 const code = typeof error === 'object' && error && 'code' in error ? String(error.code) : '';
 const messages: Record<string, [string, string]> = {
 unauthorized:['Неверный ключ или сессия истекла.','Invalid key or expired session.'],
 invalid_schema:['Схема не прошла строгую проверку.','Schema failed strict validation.'],
 invalid_data:['Данные не соответствуют схеме.','Data does not match the schema.'],
 invalid_template:['Проверьте шаблон, обязательные поля и локаль.','Check template syntax, required fields and locale.'],
 invalid_configuration:['Проверьте локали, уникальность ключей и каналов.','Check locales and unique keys and channels.'],
 invalid_provider:['Проверьте настройки провайдера.','Check provider settings.'],
 provider_missing:['Для канала не настроен провайдер.','This channel has no configured provider.'],
 notification_unavailable:['Ключ отсутствует или выключен.','The notification key is missing or disabled.'],
 expired:['Срок доставки истёк. Повтор запрещён.','Delivery expired. Replay is prohibited.'],
 rate_limited:['Слишком много попыток. Подождите минуту.','Too many attempts. Wait a minute.'],
 idempotency_conflict:['Ключ идемпотентности уже использован для другого содержимого.','Idempotency key already belongs to different content.'],
 origin_rejected:['Адрес страницы не совпадает с настроенным адресом Courier.','Page origin does not match the configured Courier origin.'],
 invalid_request:['Заполните обязательные поля и проверьте формат значений.','Fill required fields and check value formats.'],
 };
 const message = messages[code];
 notice(message ? t(...message) : t('Не удалось выполнить действие. Проверьте введённые данные и соединение.','Action failed. Check input and connection.'),true);
}
export function notice(message: string, error = false) {
 const node = document.getElementById('notice');
 if (node) { node.textContent = message; node.className = error ? 'notice error' : 'notice'; node.hidden = false; }
}
