package openapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const refOpenAPITestDocument = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Ref API",
    "version": "1.0.0"
  },
  "servers": [
    { "url": "https://api.example.test" }
  ],
  "components": {
    "parameters": {
      "PetId": {
        "name": "petId",
        "in": "path",
        "required": true,
        "schema": { "$ref": "#/components/schemas/PetId" }
      }
    },
    "schemas": {
      "PetId": {
        "type": "string",
        "description": "Stable pet id"
      },
      "PetBase": {
        "type": "object",
        "required": ["name"],
        "properties": {
          "name": { "type": "string" }
        }
      },
      "PetCreate": {
        "allOf": [
          { "$ref": "#/components/schemas/PetBase" },
          {
            "type": "object",
            "properties": {
              "tag": { "type": "string" }
            }
          }
        ]
      }
    },
    "requestBodies": {
      "CreatePet": {
        "required": true,
        "content": {
          "application/json": {
            "schema": { "$ref": "#/components/schemas/PetCreate" }
          }
        }
      }
    }
  },
  "paths": {
    "/pets/{petId}": {
      "parameters": [
        { "$ref": "#/components/parameters/PetId" }
      ],
      "get": {
        "operationId": "getPet",
        "responses": {
          "200": { "description": "OK" }
        }
      }
    },
    "/pets": {
      "post": {
        "operationId": "createPet",
        "requestBody": { "$ref": "#/components/requestBodies/CreatePet" },
        "responses": {
          "201": { "description": "Created" }
        }
      }
    }
  }
}`

func TestParseSpecResolvesLocalRefs(t *testing.T) {
	spec, err := ParseSpec([]byte(refOpenAPITestDocument), "https://docs.example.test/openapi.json")
	require.NoError(t, err)
	require.Len(t, spec.Operations, 2)

	operations := map[string]Operation{}
	for _, operation := range spec.Operations {
		operations[operation.Key] = operation
	}

	getPet := operations["GET /pets/{petId}"]
	require.Len(t, getPet.Parameters, 1)
	require.Equal(t, "petId", getPet.Parameters[0].Name)
	require.Equal(t, "string", getPet.Parameters[0].Schema["type"])
	require.Equal(t, "Stable pet id", getPet.Parameters[0].Schema["description"])

	getProperties := getPet.InputSchema["properties"].(map[string]any)
	petId := getProperties["petId"].(map[string]any)
	require.NotContains(t, petId, "$ref")
	require.Equal(t, "string", petId["type"])

	createPet := operations["POST /pets"]
	bodyProperties := createPet.RequestBodySchema["properties"].(map[string]any)
	require.Contains(t, bodyProperties, "name")
	require.Contains(t, bodyProperties, "tag")
	require.ElementsMatch(t, []any{"name"}, createPet.RequestBodySchema["required"])

	inputProperties := createPet.InputSchema["properties"].(map[string]any)
	body := inputProperties["body"].(map[string]any)
	require.NotContains(t, body, "$ref")
	require.Contains(t, body["properties"], "name")
	require.Contains(t, body["properties"], "tag")
	require.ElementsMatch(t, []any{"body"}, createPet.InputSchema["required"])
}

const complexSchemaOpenAPITestDocument = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Complex Schema API",
    "version": "1.0.0"
  },
  "servers": [
    { "url": "https://api.example.test" }
  ],
  "components": {
    "schemas": {
      "Identifier": {
        "type": "string",
        "description": "Stable identifier"
      },
      "Label": {
        "type": "string",
        "description": "Human readable label"
      },
      "BaseItem": {
        "type": "object",
        "required": ["id"],
        "properties": {
          "id": { "$ref": "#/components/schemas/Identifier" },
          "labels": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/Label" }
          },
          "metadata": {
            "type": "object",
            "additionalProperties": { "$ref": "#/components/schemas/Label" }
          }
        }
      },
      "CreateItem": {
        "allOf": [
          { "$ref": "#/components/schemas/BaseItem" },
          {
            "type": "object",
            "required": ["status"],
            "properties": {
              "id": {
                "minLength": 3
              },
              "labels": {
                "items": {
                  "minLength": 2
                }
              },
              "metadata": {
                "additionalProperties": {
                  "minLength": 2
                }
              },
              "status": {
                "type": "string",
                "enum": ["draft", "active"]
              }
            }
          }
        ]
      }
    }
  },
  "paths": {
    "/items": {
      "post": {
        "operationId": "createItem",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/CreateItem" }
            }
          }
        },
        "responses": {
          "201": { "description": "Created" }
        }
      }
    }
  }
}`

func TestParseSpecMergesComplexAllOfPropertySchemas(t *testing.T) {
	spec, err := ParseSpec([]byte(complexSchemaOpenAPITestDocument), "https://docs.example.test/openapi.json")
	require.NoError(t, err)
	require.Len(t, spec.Operations, 1)

	operation := spec.Operations[0]
	bodyProperties := operation.RequestBodySchema["properties"].(map[string]any)
	require.ElementsMatch(t, []any{"id", "status"}, operation.RequestBodySchema["required"])

	id := bodyProperties["id"].(map[string]any)
	require.Equal(t, "string", id["type"])
	require.Equal(t, "Stable identifier", id["description"])
	require.EqualValues(t, 3, id["minLength"])

	labels := bodyProperties["labels"].(map[string]any)
	labelItems := labels["items"].(map[string]any)
	require.Equal(t, "string", labelItems["type"])
	require.Equal(t, "Human readable label", labelItems["description"])
	require.EqualValues(t, 2, labelItems["minLength"])

	metadata := bodyProperties["metadata"].(map[string]any)
	additionalProperties := metadata["additionalProperties"].(map[string]any)
	require.Equal(t, "string", additionalProperties["type"])
	require.Equal(t, "Human readable label", additionalProperties["description"])
	require.EqualValues(t, 2, additionalProperties["minLength"])
}

const formOpenAPITestDocument = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Form API",
    "version": "1.0.0"
  },
  "servers": [
    { "url": "https://api.example.test" }
  ],
  "paths": {
    "/oauth/token": {
      "post": {
        "operationId": "createToken",
        "requestBody": {
          "required": true,
          "content": {
            "application/x-www-form-urlencoded": {
              "schema": {
                "type": "object",
                "required": ["grant_type"],
                "properties": {
                  "grant_type": { "type": "string" },
                  "scope": {
                    "type": "array",
                    "items": { "type": "string" }
                  }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "description": "OK" }
        }
      }
    },
    "/uploads": {
      "post": {
        "operationId": "uploadAsset",
        "requestBody": {
          "content": {
            "multipart/form-data": {
              "schema": {
                "type": "object",
                "properties": {
                  "title": { "type": "string" },
                  "file": { "type": "string", "format": "binary" }
                }
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Created" }
        }
      }
    }
  }
}`

func TestParseSpecPreservesFormAndMultipartRequestBodies(t *testing.T) {
	spec, err := ParseSpec([]byte(formOpenAPITestDocument), "https://docs.example.test/openapi.json")
	require.NoError(t, err)
	require.Len(t, spec.Operations, 2)

	operations := map[string]Operation{}
	for _, operation := range spec.Operations {
		operations[operation.Key] = operation
	}

	token := operations["POST /oauth/token"]
	require.Equal(t, "application/x-www-form-urlencoded", token.RequestContentType)
	require.ElementsMatch(t, []any{"grant_type"}, token.RequestBodySchema["required"])
	tokenInputProperties := token.InputSchema["properties"].(map[string]any)
	tokenBody := tokenInputProperties["body"].(map[string]any)
	require.Equal(t, "object", tokenBody["type"])
	require.Contains(t, tokenBody["properties"], "grant_type")
	require.Contains(t, tokenBody["properties"], "scope")
	require.ElementsMatch(t, []any{"body"}, token.InputSchema["required"])

	upload := operations["POST /uploads"]
	require.Equal(t, "multipart/form-data", upload.RequestContentType)
	uploadProperties := upload.RequestBodySchema["properties"].(map[string]any)
	file := uploadProperties["file"].(map[string]any)
	require.Equal(t, "string", file["type"])
	require.Equal(t, "binary", file["format"])

	uploadInputProperties := upload.InputSchema["properties"].(map[string]any)
	uploadBody := uploadInputProperties["body"].(map[string]any)
	uploadBodyProperties := uploadBody["properties"].(map[string]any)
	inputFile := uploadBodyProperties["file"].(map[string]any)
	require.Equal(t, "object", inputFile["type"])
	require.Equal(t, true, inputFile["x-openapi-file-upload"])
	require.ElementsMatch(t, []any{"content_base64"}, inputFile["required"])
	inputFileProperties := inputFile["properties"].(map[string]any)
	require.Contains(t, inputFileProperties, "filename")
	require.Contains(t, inputFileProperties, "content_type")
	require.Contains(t, inputFileProperties, "content_base64")
}

const binaryBodyOpenAPITestDocument = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Binary API",
    "version": "1.0.0"
  },
  "servers": [
    { "url": "https://api.example.test" }
  ],
  "paths": {
    "/raw": {
      "post": {
        "operationId": "uploadRaw",
        "requestBody": {
          "required": true,
          "content": {
            "application/octet-stream": {
              "schema": {
                "type": "string",
                "format": "binary",
                "description": "Raw file bytes"
              }
            }
          }
        },
        "responses": {
          "201": { "description": "Created" }
        }
      }
    }
  }
}`

func TestParseSpecUsesBinaryRequestBodyInputSchema(t *testing.T) {
	spec, err := ParseSpec([]byte(binaryBodyOpenAPITestDocument), "https://docs.example.test/openapi.json")
	require.NoError(t, err)
	require.Len(t, spec.Operations, 1)

	operation := spec.Operations[0]
	require.Equal(t, "application/octet-stream", operation.RequestContentType)
	require.Equal(t, "binary", operation.RequestBodySchema["format"])

	inputProperties := operation.InputSchema["properties"].(map[string]any)
	body := inputProperties["body"].(map[string]any)
	require.Equal(t, "object", body["type"])
	require.Equal(t, true, body["x-openapi-binary-body"])
	require.ElementsMatch(t, []any{"content_base64"}, body["required"])
	bodyProperties := body["properties"].(map[string]any)
	require.Contains(t, bodyProperties, "content_type")
	require.Contains(t, bodyProperties, "content_base64")
	require.ElementsMatch(t, []any{"body"}, operation.InputSchema["required"])
}
