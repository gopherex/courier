# Schemas, templates and localization

Each channel owns a schemapb schema in protobuf JSON format. The visual editor
supports basic named fields and translated labels; the full JSON editor supports
nested objects, lists, constraints and validation rules.

```json
{
  "id": {"name": "registration"},
  "strict": true,
  "fields": [{"name": "name", "required": true, "string": {}}]
}
```

All nested schemas must be strict. Unknown fields, wrong types and missing required
fields reject the request. Defaults, computed fields, coercion, normalization and
conditional field gates are prohibited. CEL validation has a cost limit of 10,000.
The Go runtime is authoritative; the browser uses the same pinned schemapb engine
for feedback. JSON schemas here are schemapb definitions, not JSON Schema drafts.

Templates use Go `text/template` and `html/template`, with `missingkey=error`:

```text
Welcome, {{.name}}
```

Email needs a subject and text or HTML. HTML values are escaped automatically.
Push needs a title and body; an optional URL must render to HTTPS. Reply-To must
be a valid address, and email headers cannot contain CR/LF. Rendered fields are
limited to 256 KiB; a request body is limited to 1 MiB.

Locale resolution tries the exact canonical BCP-47 tag, then base language, then
project default. For example, `pt-BR` tries `pt-BR`, `pt`, then the default locale.
The complete template is selected together, not field by field. Every channel
must have a complete template for the project's default locale.

Templates are rendered before acceptance and frozen in the job. Editing a template
affects new requests only. Use preview to check rendering without queuing; test
delivery follows the same durable admission and provider path as normal delivery.
