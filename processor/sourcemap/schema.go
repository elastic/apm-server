package sourcemap

func Schema() string {
	return sourcemapSchema
}

var sourcemapSchema = `{
    "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/sourcemaps/sourcemap-wrapper.json",
    "title": "Sourcemap Wrapper",
    "description": "Sourcemap + Metadata",
    "type": "object",
    "properties": {
        "bundle_filepath": {
            "description": "relative path of the minified bundle file",
            "type": "string",
            "maxLength": 1024,
            "minLength": 1
        },
        "app": {
            "allOf": [
                {     "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "doc/spec/app_name.json",
    "title": "App name property",
    "type": "object",
    "properties": {
        "name": {
            "description": "Immutable name of the app emitting this event",
            "type": "string",
            "pattern": "^[a-zA-Z0-9 _-]+$",
            "maxLength": 1024
        }
    },
    "required": ["name"] },
                { "properties": {
                    "version": {
                        "description": "Version of the app emitting this event",
                        "type": "string",
                        "maxLength": 1024,
                        "minLength": 1
                    }
                },
                "required": ["version"]
                }
            ]
        },
        "sourcemap": {
            "type": "object",
                "$schema": "http://json-schema.org/draft-04/schema#",
    "$id": "docs/spec/sourcemaps/sourcemap.json",
    "title": "Sourcemap",
    "description": "Sourcemap",
    "type": "object",
    "properties": {
        "version": {
            "type": "number"
        },
        "sources": {
            "type": "array",
            "items": {
                "type": "string"
            }
        },
        "names": {
            "type": "array",
            "items": {
                "type": "string"
            }
        },
        "mappings": {
            "type": "string"
        },
        "file": {
            "type": ["string", "null"]
        },
        "sourceRoot": {
            "type": ["string", "null"]
        },
        "sourcesContent": {
            "type": ["array", "null"],
            "items": {
                "type": ["string", "null"]
            },
            "minItems": 0
        }
    },
    "required": ["version", "sources", "names", "mappings"]
        }
    },
    "required": ["bundle_filepath", "sourcemap"]
}
`
