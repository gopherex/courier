import { fromJson, toJson, type JsonObject } from '@bufbuild/protobuf';
import { Engine, SchemaSchema, Schema_FieldSchema } from '@gopherex/schemapb';
import type { ChannelConfig, Data } from '@gopherex/courier-sdk';
import { escape, t, value } from './ui';

export function schemaEditor(channel: ChannelConfig): string {
 const schema = fromJson(SchemaSchema, channel.schema as JsonObject);
 return `<div class="section-title"><h3>${t('Схема данных','Data schema')}</h3><button id="add-field" class="quiet">+ ${t('Поле','Field')}</button></div>
 <p class="hint">${t('Неизвестные поля запрещены. Данные не преобразуются.','Unknown fields are rejected. Data is never transformed.')}</p>
 <div class="field-table"><div class="field-head" aria-hidden="true"><span>${t('Имя','Name')}</span><span>${t('Тип','Type')}</span><span>${t('Обязательность','Required')}</span><span>RU</span><span>EN</span><span></span></div>${schema.fields.length?'':`<p class="field-empty">${t('Поля не заданы. Добавьте поле или загрузите JSON-схему.','No fields. Add a field or load a JSON schema.')}</p>`}${schema.fields.map((field,index) => `<div class="field-row" data-field="${index}">
 <input aria-label="${t('Имя поля','Field name')}" data-name value="${escape(field.name)}" required>
 <select aria-label="${t('Тип','Type')}" data-kind>${['string','bool','int32','double', ...(!['string','bool','int32','double'].includes(field.kind.case ?? '') ? [field.kind.case ?? ''] : [])].map(kind=>`<option ${kind===field.kind.case?'selected':''}>${kind}</option>`).join('')}</select>
 <label class="check"><input data-required type="checkbox" ${field.required?'checked':''}>${t('Обязательное','Required')}</label>
 <input data-label-ru aria-label="Название RU" placeholder="Название RU" value="${escape(channel.labels?.[field.name]?.ru ?? '')}">
 <input data-label-en aria-label="Label EN" placeholder="Label EN" value="${escape(channel.labels?.[field.name]?.en ?? '')}">
 <button class="quiet danger" data-delete-field="${index}" aria-label="${t('Удалить поле','Delete field')}">×</button></div>`).join('')}</div>
 <details><summary>${t('Полная схема JSON','Full schema JSON')}</summary><p class="hint">${t('Для вложенных объектов, списков, ограничений и правил. Применение заменяет схему в форме.','For nested objects, lists, constraints and rules. Applying replaces the form schema.')}</p>
 <textarea id="schema-json" rows="10" spellcheck="false">${escape(JSON.stringify(channel.schema,null,2))}</textarea><button id="apply-schema">${t('Применить JSON','Apply JSON')}</button></details>`;
}
export function readSchema(channel: ChannelConfig) {
 const schema = fromJson(SchemaSchema, channel.schema as JsonObject);
 channel.labels ??= {};
 document.querySelectorAll<HTMLElement>('[data-field]').forEach(row=>{
  const index = Number(row.dataset.field);
  const original=schema.fields[index];
  const name=row.querySelector<HTMLInputElement>('[data-name]')!.value;
  const kind=row.querySelector<HTMLSelectElement>('[data-kind]')!.value;
  let field=original;
  if (kind!==original.kind.case) field=fromJson(Schema_FieldSchema,{name,[kind]:{}});
  field.name=name;field.required=row.querySelector<HTMLInputElement>('[data-required]')!.checked;
  schema.fields[index]=field;
  channel.labels![name]={ru:row.querySelector<HTMLInputElement>('[data-label-ru]')!.value,en:row.querySelector<HTMLInputElement>('[data-label-en]')!.value};
 });
 channel.schema=toJson(SchemaSchema,schema) as JsonObject;
 Engine.compile(schema);
}
export function addField(channel: ChannelConfig) {
 const schema=fromJson(SchemaSchema,channel.schema as JsonObject);
 schema.fields.push(fromJson(Schema_FieldSchema,{name:`field_${schema.fields.length+1}`,required:true,string:{}}));
 channel.schema=toJson(SchemaSchema,schema) as JsonObject;
}
export function deleteField(channel: ChannelConfig,index: number) {
 const schema=fromJson(SchemaSchema,channel.schema as JsonObject);schema.fields.splice(index,1);
 channel.schema=toJson(SchemaSchema,schema) as JsonObject;
}
export function applySchema(channel: ChannelConfig) {
 const schema=fromJson(SchemaSchema,JSON.parse(value('schema-json')));Engine.compile(schema);
 channel.schema=toJson(SchemaSchema,schema) as JsonObject;
}
export function validateData(channel: ChannelConfig,data: Data): boolean {
 const schema=fromJson(SchemaSchema,channel.schema as JsonObject);
 return Engine.compile(schema).validate(data as Parameters<Engine['validate']>[0]).errors.length===0;
}
