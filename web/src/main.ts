import * as api from '@gopherex/courier-sdk';
import { language, setLanguage, t, escape as e, input, area, value, checked, on, showError, notice } from './ui';
import { schemaEditor, readSchema, addField, deleteField, applySchema, validateData } from './schema';
import './style.css';

api.client.setConfig({ baseUrl: location.origin, credentials: 'same-origin', throwOnError: true });
const root = document.getElementById('app')!;
let projects: api.Project[] = [];
let selected: api.Project | undefined;
let notificationIndex = 0;
let channelIndex = 0;
let templateIndex = 0;
let tab = 'notifications';
let draft: api.ProjectConfig | undefined;
let editorTab = 'general';

const languageButton = () => `<button id="language" class="quiet" aria-label="${t('Сменить язык','Change language')}">${language === 'ru' ? 'EN' : 'RU'}</button>`;
function bindLanguage(action: () => void) { on('language', () => { setLanguage(language === 'ru' ? 'en' : 'ru'); action(); }); }
function current() { if (!selected || !draft) throw new Error('No project'); return { project: selected, config: draft }; }
const path = () => ({ project_id: current().project.id });
const save = async () => { await api.putProject({ path: path(), body: current().config }); selected!.config = structuredClone(draft!); notice(t('Изменения действуют.','Changes are live.')); };

function loginPage() {
 root.innerHTML = `<main class="login"><header><span class="brand"><img class="brand-mark" src="/logo.svg" alt="" width="24" height="24">courier <span class="console-label">Console</span></span>${languageButton()}</header><section class="login-body"><div class="login-heading"><h1>${t('Вход в консоль','Sign in to console')}</h1><p>${t('Администрирование службы уведомлений','Notification service administration')}</p></div><form id="login-form">${input('master-key',t('Ключ администратора','Administrator key'),'','password','required minlength="32" autocomplete="current-password"')}<button class="primary">${t('Войти','Sign in')}</button></form><p id="notice" class="notice" role="status" hidden></p></section></main>`;
 bindLanguage(loginPage);
 document.getElementById('login-form')!.addEventListener('submit', event => {
  event.preventDefault(); const key = value('master-key');
  api.login({body:{key}}).then(load).catch(showError);
 });
}
async function load() {
 const response = await api.listProjects(); projects = response.data!.items;
 if (selected) selected = projects.find(project => project.id === selected!.id);
 selected ??= projects[0]; draft = selected ? structuredClone(selected.config) : undefined;
 shell();
}
function shell() {
 const navigation = [
  ['notifications',t('Уведомления','Notifications'),'notification'],
  ['providers',t('Провайдеры','Providers'),'provider'],
  ['keys',t('Доступ сервисов','Service access'),'key'],
  ['dead',t('Ошибки доставки','Dead letters'),'dead'],
  ['project',t('Настройки проекта','Project settings'),'settings'],
 ];
 root.innerHTML = `<a class="skip-link" href="#content">${t('К содержимому','Skip to content')}</a><header class="appbar"><div class="brand"><img class="brand-mark" src="/logo.svg" alt="" width="24" height="24">courier<span class="console-label">Console</span></div><div class="appbar-actions"><span class="session-label">${t('Администратор','Administrator')}</span>${languageButton()}</div></header><div class="layout"><aside class="sidebar"><div class="project-picker"><label for="project-switcher">${t('Проект','Project')}</label><select id="project-switcher">${projects.length?projects.map(project=>`<option value="${e(project.id)}" ${project.id===selected?.id?'selected':''}>${e(project.config.title[language] || project.id)}</option>`).join(''):`<option>${t('Нет проектов','No projects')}</option>`}</select><button id="new-project" class="quiet add">+ ${t('Новый проект','New project')}</button></div>${selected?`<nav class="navigation" aria-label="${t('Разделы проекта','Project navigation')}">${navigation.map(([key,label,icon])=>`<button data-tab="${key}" class="${tab===key?'active':''}" ${tab===key?'aria-current="page"':''}><span class="nav-icon ${icon}" aria-hidden="true"></span>${label}</button>`).join('')}</nav>`:''}<div class="sidebar-bottom"><span class="session-label">${t('Сессия администратора','Administrator session')}</span><button id="logout" class="quiet">${t('Выйти','Sign out')}</button></div></aside><main class="workspace"><header class="topbar"><div class="breadcrumb">${t('Проекты','Projects')}<span>/</span><code>${e(selected?.id || '—')}</code></div><h1>${e(selected?.config.title[language] || selected?.id || t('Проекты','Projects'))}</h1></header><div id="notice" class="notice" role="status" hidden></div><section id="content" tabindex="-1"></section></main></div>`;
 bindLanguage(shell);
 on('logout',async()=>{await api.logout(); selected=undefined; projects=[]; loginPage();});
 on('new-project',newProject);
 document.getElementById('project-switcher')!.onchange=()=>{
  selected=projects.find(project=>project.id===value('project-switcher')); draft=selected?structuredClone(selected.config):undefined;
  notificationIndex=channelIndex=templateIndex=0; shell();
 };
 document.querySelectorAll<HTMLElement>('[data-tab]').forEach(button=>button.onclick=()=>{ tab=button.dataset.tab!;shell(); });
 if (!selected) { content(`<div class="empty"><h2>${t('Нет проектов','No projects')}</h2><p>${t('Создайте проект для настройки уведомлений и подключения провайдеров.','Create a project to configure notifications and connect providers.')}</p><button id="start" class="primary">${t('Создать проект','Create project')}</button></div>`);on('start',newProject);return; }
 void ({notifications:notificationsPage,providers:providersPage,keys:keysPage,dead:deadPage,project:projectPage}[tab] ?? notificationsPage)();
}
function content(html: string) { document.getElementById('content')!.innerHTML=html; }
function newProject() {
 document.querySelector('.topbar')!.innerHTML=`<div class="breadcrumb">${t('Проекты','Projects')}</div><h1>${t('Новый проект','New project')}</h1>`;
 document.querySelectorAll('[data-tab]').forEach(button=>{button.classList.remove('active');button.removeAttribute('aria-current');});
 content(`<div class="narrow"><h2>${t('Новый проект','New project')}</h2><p class="hint">${t('Отдельные ключи доступа, настройки каналов и предпочтения получателей.','Independent access keys, channel settings and recipient preferences.')}</p>${input('project-id',t('Технический идентификатор','Project identifier'),'','','required pattern="[a-zA-Z0-9][a-zA-Z0-9_.-]*"')}${input('project-ru','Название · RU')}${input('project-en','Title · EN')}${input('project-locale',t('Основная локаль','Default locale'),'ru')}<button id="create-project" class="primary">${t('Создать проект','Create project')}</button> <button id="cancel-project">${t('Отмена','Cancel')}</button></div>`);
 on('cancel-project',shell);
 on('create-project',async()=>{
  const id=value('project-id');await api.putProject({path:{project_id:id},body:{title:{ru:value('project-ru'),en:value('project-en')},default_locale:value('project-locale'),notifications:[]}});
  selected=undefined;await load();selected=projects.find(project=>project.id===id);draft=structuredClone(selected!.config);shell();
 });
}
function projectPage() {
 const config=current().config;
 content(`<div class="narrow"><h2>${t('Настройки проекта','Project settings')}</h2>${input('project-ru','Название · RU',config.title.ru)}${input('project-en','Title · EN',config.title.en)}${input('project-locale',t('Основная локаль','Default locale'),config.default_locale)}<p class="hint">${t('Для каждого канала нужен полный шаблон на основной локали.','Every channel needs a complete template in the default locale.')}</p><button id="save-project" class="primary">${t('Сохранить','Save')}</button></div>`);
 on('save-project',async()=>{config.title={...config.title,ru:value('project-ru'),en:value('project-en')};config.default_locale=value('project-locale');await save();});
}
function newChannel(channel: api.Channel): api.ChannelConfig {
 return {channel,schema:{id:{name:'notification'},strict:true,fields:[]},templates:[{locale:current().config.default_locale,...(channel==='email'?{subject:'',text:''}:{title:'',body:''})}]};
}
function notificationsPage() {
 const config=current().config;
 const notification=config.notifications[notificationIndex];
 content(`<div class="section-title"><div><h2>${t('Ключи уведомлений','Notification keys')}</h2><p class="hint">${t('Сохранение сразу меняет обработку новых сообщений.','Saving immediately changes how new messages are processed.')}</p></div><button id="new-notification" class="primary">+ ${t('Добавить ключ','Add key')}</button></div><div class="editor-layout"><nav class="key-list" aria-label="${t('Ключи уведомлений','Notification keys')}"><div class="list-heading">${t('Ключ','Key')}<span class="count">${config.notifications.length}</span></div>${config.notifications.map((item,index)=>`<button data-notification="${index}" class="${index===notificationIndex?'selected':''}"><span class="key-name"><span class="state-dot ${item.active?'enabled':''}" title="${item.active?t('Активен','Active'):t('Выключен','Disabled')}"></span><code>${e(item.key)}</code></span><small>${e(item.title[language]||t('Без названия','Untitled'))}</small></button>`).join('')}</nav><div id="notification-editor">${notification?'':`<div class="empty"><h3>${t('Пока нет ключей','No keys yet')}</h3><p>${t('Например: registration, password_reset или invite.','For example: registration, password_reset or invite.')}</p></div>`}</div></div>`);
 on('new-notification',()=>{
  document.getElementById('notification-editor')!.innerHTML=`<h3>${t('Новый ключ','New key')}</h3>${input('new-key',t('Ключ','Key'),'registration')}<button id="add-key" class="primary">${t('Добавить','Add')}</button>`;
  on('add-key',()=>{config.notifications.push({key:value('new-key'),active:true,title:{ru:'',en:''},description:{ru:'',en:''},channels:[newChannel('email')]});notificationIndex=config.notifications.length-1;channelIndex=templateIndex=0;editorTab='general';notificationsPage();});
 });
 document.querySelectorAll<HTMLElement>('[data-notification]').forEach(button=>button.onclick=()=>{notificationIndex=Number(button.dataset.notification);channelIndex=templateIndex=0;editorTab='general';notificationsPage();});
 if (notification) editNotification(notification);
}
function editNotification(notification: api.Notification) {
 const channel=notification.channels[channelIndex] ?? notification.channels[0];
 const template=channel.templates[templateIndex] ?? channel.templates[0];
 document.getElementById('notification-editor')!.innerHTML=`<div class="editor-toolbar"><h3><code>${e(notification.key)}</code></h3><label class="check"><input id="key-active" type="checkbox" ${notification.active?'checked':''}>${t('Активен','Active')}</label><button id="save-notification" class="primary">${t('Сохранить настройки','Save configuration')}</button></div><nav class="editor-tabs" aria-label="${t('Редактор уведомления','Notification editor')}">${[['general',t('Настройки','Settings')],['schema',t('Схема данных','Data schema')],['templates',t('Шаблоны','Templates')],['test',t('Проверка','Test delivery')]].map(([key,label])=>`<button data-editor-tab="${key}" class="${editorTab===key?'active':''}" aria-pressed="${editorTab===key}">${label}</button>`).join('')}</nav><section data-editor-pane="general" ${editorTab==='general'?'':'hidden'}><div class="pane-heading"><h3>${t('Настройки уведомления','Notification settings')}</h3><p class="hint">${t('Названия и описания для каждой локали. Технический ключ используется при отправке.','Localized names and descriptions. Use the technical key when sending messages.')}</p></div><div class="two">${input('title-ru','Название · RU',notification.title.ru)}${input('title-en','Title · EN',notification.title.en)}${area('description-ru','Описание · RU',notification.description.ru,2)}${area('description-en','Description · EN',notification.description.en,2)}</div></section><div class="channel-tabs" ${editorTab==='general'?'hidden':''}><span class="channel-label">${t('Канал','Channel')}</span>${notification.channels.map((item,index)=>`<button data-channel="${index}" class="${item===channel?'active':''}">${item.channel}</button>`).join('')}<select id="add-channel" aria-label="${t('Добавить канал','Add channel')}"><option value="">+ ${t('Канал','Channel')}</option>${(['email','webpush','fcm'] as const).filter(kind=>!notification.channels.some(item=>item.channel===kind)).map(kind=>`<option>${kind}</option>`).join('')}</select></div><section data-editor-pane="schema" ${editorTab==='schema'?'':'hidden'}>${schemaEditor(channel)}</section><section data-editor-pane="templates" ${editorTab==='templates'?'':'hidden'}><div class="section-title"><h3>${t('Шаблоны','Templates')}</h3><div class="inline"><select id="template-locale" aria-label="${t('Локаль шаблона','Template locale')}">${channel.templates.map((item,index)=>`<option value="${index}" ${item===template?'selected':''}>${e(item.locale)}</option>`).join('')}</select><input id="new-locale" aria-label="${t('Новая локаль','New locale')}" placeholder="en-US"><button id="add-locale" aria-label="${t('Добавить локаль','Add locale')}">+</button></div></div>${channel.channel==='email'?`${input('subject',t('Тема','Subject'),template.subject)}${area('text',t('Текст','Text'),template.text,6)}${area('html','HTML',template.html,6)}${input('reply-to','Reply-To',template.reply_to)}`:`${input('push-title',t('Заголовок','Title'),template.title)}${area('push-body',t('Текст','Body'),template.body)}${input('push-url','URL',template.url)}`}<p class="hint">${t('Данные доступны как {{.name}}. HTML автоматически экранируется.','Access data as {{.name}}. HTML is automatically escaped.')}</p></section><section class="preview" data-editor-pane="test" ${editorTab==='test'?'':'hidden'}><h3>${t('Проверка и тестовая отправка','Preview and test delivery')}</h3><p class="hint">${t('Используется сохранённая конфигурация. Сначала сохраните изменения.','Uses saved configuration. Save your changes first.')}</p>${area('sample-data',t('Данные для проверки · JSON','Sample data · JSON'),'{}',4)}<button id="preview">${t('Предпросмотр','Preview')}</button><div id="preview-result"></div>${area('test-target',t('Адрес канала · JSON','Channel target · JSON'),channel.channel==='email'?'{"address":""}':channel.channel==='fcm'?'{"token":""}':'{"endpoint":"","p256dh":"","auth":""}',3)}<button id="test-send">${t('Отправить тестовое сообщение','Send test message')}</button></section>`;
 const collect=()=>{
  notification.active=checked('key-active');notification.title={...notification.title,ru:value('title-ru'),en:value('title-en')};notification.description={...notification.description,ru:value('description-ru'),en:value('description-en')};readSchema(channel);
  if(channel.channel==='email'){template.subject=value('subject');template.text=value('text');template.html=value('html');template.reply_to=value('reply-to');}
  else{template.title=value('push-title');template.body=value('push-body');template.url=value('push-url');}
 };
 document.querySelectorAll<HTMLButtonElement>('[data-editor-tab]').forEach(button=>button.onclick=()=>{
  editorTab=button.dataset.editorTab!;
  document.querySelectorAll<HTMLButtonElement>('[data-editor-tab]').forEach(item=>{
   const active=item.dataset.editorTab===editorTab;item.classList.toggle('active',active);item.setAttribute('aria-pressed',String(active));
  });
  document.querySelectorAll<HTMLElement>('[data-editor-pane]').forEach(pane=>pane.hidden=pane.dataset.editorPane!==editorTab);
  document.querySelector<HTMLElement>('.channel-tabs')!.hidden=editorTab==='general';
 });
 on('save-notification',async()=>{
  collect();await save();
  const item=document.querySelector<HTMLElement>(`[data-notification="${notificationIndex}"]`);
  if(item){
   item.querySelector('small')!.textContent=notification.title[language]||t('Без названия','Untitled');
   const state=item.querySelector<HTMLElement>('.state-dot')!;
   state.classList.toggle('enabled',notification.active);state.title=notification.active?t('Активен','Active'):t('Выключен','Disabled');
  }
 });
 document.querySelectorAll<HTMLElement>('[data-channel]').forEach(button=>button.onclick=()=>{try{collect();channelIndex=Number(button.dataset.channel);templateIndex=0;editNotification(notification);}catch(error){showError(error);}});
 document.getElementById('add-channel')!.onchange=()=>{collect();const kind=value('add-channel') as api.Channel;if(kind){notification.channels.push(newChannel(kind));channelIndex=notification.channels.length-1;templateIndex=0;editNotification(notification);}};
 document.getElementById('template-locale')!.onchange=()=>{collect();templateIndex=Number(value('template-locale'));editNotification(notification);};
 on('add-locale',()=>{collect();const locale=value('new-locale');if(locale){channel.templates.push({...template,locale});templateIndex=channel.templates.length-1;editNotification(notification);}});
 on('add-field',()=>{collect();addField(channel);editNotification(notification);});
 document.querySelectorAll<HTMLElement>('[data-delete-field]').forEach(button=>button.onclick=()=>{collect();deleteField(channel,Number(button.dataset.deleteField));editNotification(notification);});
 on('apply-schema',()=>{applySchema(channel);editNotification(notification);});
 on('preview',async()=>{
  const data: api.Data=JSON.parse(value('sample-data'));
  if(!validateData(channel,data)) { notice(t('Проверка в браузере: данные не соответствуют схеме.','Browser validation: data does not match the schema.'),true);return; }
  const response=await api.preview({path:path(),body:{notification_key:notification.key,channel:channel.channel,locale:template.locale,data}});
  const result=response.data!;
  document.getElementById('preview-result')!.innerHTML=`<h4>${e(result.subject||result.title)}</h4><pre>${e(result.text||result.body)}</pre>${result.html?'<iframe id="html-preview" sandbox="" referrerpolicy="no-referrer" title="HTML preview"></iframe>':''}`;
  const frame=document.getElementById('html-preview') as HTMLIFrameElement|null;
  if(frame)frame.srcdoc=`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; img-src data:">${result.html}`;
 });
 on('test-send',async()=>{
  const target=JSON.parse(value('test-target'));const delivery:api.DeliveryInput={channel:channel.channel,default_enabled:true,data:JSON.parse(value('sample-data'))};
  if(channel.channel==='email')delivery.email=target;else if(channel.channel==='fcm')delivery.fcm=target;else delivery.webpush=target;
  const response=await api.testSend({path:path(),body:{notification_key:notification.key,idempotency_key:crypto.randomUUID(),locale:template.locale,deliveries:[delivery]}});
  notice(t('Тест принят в очередь: ','Test queued: ')+response.data!.message_id);
 });
}
async function providersPage() {
 try {
  const response=await api.listProviders({path:path()});
  content(`<div class="section-title"><div><h2>${t('Подключения каналов','Channel connections')}</h2><p class="hint">${t('Секреты зашифрованы. Новые настройки применяются и к ожидающим заданиям.','Secrets are encrypted. New settings also apply to pending deliveries.')}</p></div></div><div class="table-scroll"><table class="provider-list"><thead><tr><th>${t('Провайдер','Provider')}</th><th>${t('Канал','Channel')}</th><th>${t('Статус','Status')}</th><th><span class="sr-only">${t('Действия','Actions')}</span></th></tr></thead><tbody>${(['email','webpush','fcm'] as const).map(channel=>{const configured=response.data!.items.some(item=>item.channel===channel);return `<tr><td><strong>${channel==='email'?'SMTP':channel==='webpush'?'Web Push':'Firebase Cloud Messaging'}</strong></td><td><code>${channel}</code></td><td><span class="badge ${configured?'success':''}">${configured?t('Настроен','Configured'):t('Не настроен','Not configured')}</span></td><td class="row-actions"><button data-provider="${channel}">${t('Настроить','Configure')}</button></td></tr>`;}).join('')}</tbody></table></div><section id="provider-editor"></section>`);
  document.querySelectorAll<HTMLElement>('[data-provider]').forEach(button=>button.onclick=()=>providerForm(button.dataset.provider as api.Channel));
 } catch(error){showError(error);}
}
function providerForm(channel: api.Channel) {
 document.getElementById('provider-editor')!.innerHTML=`<div class="narrow"><h3>${t('Заменить настройки','Replace settings')} · ${channel}</h3><p class="hint">${t('Введите полный набор настроек. Сохранённые секреты не возвращаются в браузер.','Enter the full configuration. Stored secrets are never returned to the browser.')}</p>${channel==='email'?`${input('smtp-host','SMTP host')}${input('smtp-port',t('Порт','Port'),587,'number')}<label>TLS<select id="smtp-tls"><option value="starttls">STARTTLS</option><option value="tls">TLS</option><option value="plain">${t('Без TLS · только разработка','Plain · development only')}</option></select></label>${input('smtp-user',t('Имя пользователя','Username'))}${input('smtp-password',t('Пароль','Password'),'','password','autocomplete="new-password"')}${input('smtp-from',t('Отправитель','From'))}`:channel==='webpush'?`${input('push-subject',t('Контакт · mailto: или https:','Contact · mailto: or https:'))}${input('push-public','VAPID public key')}${input('push-private','VAPID private key','','password')}`:`${input('fcm-project','Firebase project ID')}${area('fcm-account','Service account · JSON','',10)}`}<button id="save-provider" class="primary">${t('Сохранить подключение','Save connection')}</button></div>`;
 on('save-provider',async()=>{
  const body:api.ProviderSettings={channel};
  if(channel==='email')body.smtp={host:value('smtp-host'),port:Number(value('smtp-port')),tls:value('smtp-tls') as api.SmtpSettings['tls'],username:value('smtp-user'),password:value('smtp-password'),from:value('smtp-from')};
  else if(channel==='webpush')body.webpush={subject:value('push-subject'),public_key:value('push-public'),private_key:value('push-private')};
  else body.fcm={project_id:value('fcm-project'),service_account:JSON.parse(value('fcm-account'))};
  await api.putProvider({path:path(),body});await providersPage();notice(t('Подключение сохранено.','Connection saved.'));
 });
}
async function keysPage() {
 try {
  const response=await api.listKeys({path:path()});
  content(`<h2>${t('Ключи доступа сервисов','Service access keys')}</h2><p class="hint">${t('Каждый ключ разрешает отправку и управление предпочтениями только этого проекта.','Each key allows sending and preference management for this project only.')}</p><div class="inline key-issuance">${input('key-name',t('Название ключа','Key name'),'backend')}<button id="issue-key" class="primary">${t('Выпустить ключ','Issue key')}</button></div><div id="issued-key"></div><table><thead><tr><th>${t('Название','Name')}</th><th>${t('Создан','Created')}</th><th><span class="sr-only">${t('Действия','Actions')}</span></th></tr></thead><tbody>${response.data!.items.length?'':`<tr><td colspan="3" class="table-empty">${t('Ключи доступа не выпущены','No access keys issued')}</td></tr>`}${response.data!.items.map(key=>`<tr><td>${e(key.name)}</td><td>${e(new Date(key.created_at).toLocaleString(language))}</td><td><button class="quiet danger" data-revoke="${key.id}">${t('Отозвать','Revoke')}</button></td></tr>`).join('')}</tbody></table>`);
  on('issue-key',async()=>{const result=await api.issueKey({path:path(),body:{name:value('key-name')}});await keysPage();document.getElementById('issued-key')!.innerHTML=`<div class="secret"><p>${t('Скопируйте сейчас: повторно ключ не отображается.','Copy now: this key is shown only once.')}</p><code>${e(result.data!.secret)}</code></div>`;});
  document.querySelectorAll<HTMLElement>('[data-revoke]').forEach(button=>button.onclick=()=>{if(confirm(t('Отозвать этот ключ? Сервис потеряет доступ.','Revoke this key? The service will lose access.')))api.revokeKey({path:{...path(),key_id:button.dataset.revoke!}}).then(keysPage).catch(showError);});
 }catch(error){showError(error);}
}
async function deadPage(after?: string) {
 try {
  const response=await api.listDeadLetters({path:path(),...(after?{query:{after}}:{})});const items=response.data!.items;
  content(`<div class="section-title"><div><h2>${t('Ошибки доставки','Dead letters')}</h2><p class="hint">${t('Содержимое хранится временно. Повтор использует актуальные настройки провайдера.','Content is retained temporarily. Replay uses current provider settings.')}</p></div><button id="refresh-dead">${t('Обновить','Refresh')}</button></div>${items.length?`<table><thead><tr><th>${t('Уведомление','Notification')}</th><th>${t('Канал','Channel')}</th><th>${t('Попытки','Attempts')}</th><th>${t('Причина','Reason')}</th><th></th></tr></thead><tbody>${items.map(item=>`<tr><td><strong>${e(item.snapshot.notification_key)}</strong><small>${e(new Date(item.finished_at).toLocaleString(language))}</small></td><td>${item.snapshot.channel}</td><td>${item.attempts}</td><td><code>${e(item.error_code)}</code></td><td><button data-replay="${item.id}" ${item.snapshot.expires_at&&new Date(item.snapshot.expires_at)<=new Date()?'disabled':''}>${t('Повторить','Replay')}</button><details><summary>${t('Содержимое','Payload')}</summary><pre>${e(JSON.stringify(item.snapshot,null,2))}</pre></details></td></tr>`).join('')}</tbody></table><button id="next-dead" ${items.length<100?'disabled':''}>${t('Следующие','Next')}</button>`:`<div class="empty"><h3>${t('Неудачных доставок нет','No failed deliveries')}</h3></div>`}`);
  on('refresh-dead',()=>deadPage());on('next-dead',()=>deadPage(items.at(-1)?.id));
  document.querySelectorAll<HTMLElement>('[data-replay]').forEach(button=>button.onclick=()=>{if(confirm(t('Повторить доставку? При неопределённом прошлом результате возможен дубль.','Replay delivery? An uncertain previous result may cause a duplicate.')))api.replay({path:{...path(),delivery_id:button.dataset.replay!}}).then(()=>deadPage()).catch(showError);});
 }catch(error){showError(error);}
}
load().catch(loginPage);
