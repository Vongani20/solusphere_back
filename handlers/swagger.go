package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SwaggerRoot redirects browser users to the interactive Swagger UI.
func SwaggerRoot(c *gin.Context) {
	c.Redirect(http.StatusFound, "/swagger/index.html")
}

// SwaggerIndex serves Swagger UI for the embedded OpenAPI document.
func SwaggerIndex(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerIndexHTML))
}

// SwaggerSpec serves the OpenAPI document used by Swagger UI.
func SwaggerSpec(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(openAPISpecJSON))
}

const swaggerIndexHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Solusphere API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
  <style>
    body { margin: 0; background: #f6f7f9; }
    .topbar { display: none; }
    #swagger-ui .scheme-container { border-radius: 0; box-shadow: none; }
    .docs-fallback {
      padding: 12px 16px;
      background: #111827;
      color: #fff;
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      font-size: 14px;
    }
    .docs-fallback a { color: #93c5fd; }
  </style>
</head>
<body>
  <div class="docs-fallback">Solusphere API documentation. Raw spec: <a href="/swagger/doc.json">/swagger/doc.json</a></div>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.addEventListener("load", function () {
      SwaggerUIBundle({
        url: "/swagger/doc.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        persistAuthorization: true,
        displayRequestDuration: true
      });
    });
  </script>
</body>
</html>`

const openAPISpecJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Solusphere Backend API",
    "version": "1.0.0",
    "description": "Authentication, face recognition, helpdesk, event chat, admin, file upload, chatbot, and BPO PDF analysis endpoints for Solusphere."
  },
  "servers": [
    {
      "url": "http://localhost:2080",
      "description": "Local development server"
    }
  ],
  "tags": [
    { "name": "Health" },
    { "name": "Authentication" },
    { "name": "Profile" },
    { "name": "Face Recognition" },
    { "name": "Helpdesk" },
    { "name": "Events" },
    { "name": "Admin" },
    { "name": "AI" },
    { "name": "BPO Analysis" },
    { "name": "CV Builder" },
    { "name": "Uploads" },
    { "name": "Debug" }
  ],
  "paths": {
    "/health": {
      "get": {
        "tags": ["Health"],
        "summary": "Check API, database, AWS, and OpenAI configuration status",
        "responses": {
          "200": {
            "description": "Server status",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/HealthResponse" }
              }
            }
          }
        }
      }
    },
    "/debug/db": {
      "get": {
        "tags": ["Debug"],
        "summary": "Inspect database connection details",
        "responses": {
          "200": { "$ref": "#/components/responses/Success" }
        }
      }
    },
    "/debug/ping": {
      "get": {
        "tags": ["Debug"],
        "summary": "Ping the database",
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/auth/register": {
      "post": {
        "tags": ["Authentication"],
        "summary": "Register a new user",
        "description": "New users must log in with email/password, then complete face registration with POST /api/face/register before using protected app endpoints.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/RegisterRequest" },
              "example": {
                "username": "demo",
                "email": "demo@example.com",
                "phone_number": "+15551234567",
                "password": "password123"
              }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/auth/outlook365/signup": {
      "post": {
        "tags": ["Authentication"],
        "summary": "Register an Outlook 365 account",
        "description": "Creates a user with auth_provider set to outlook365. Microsoft 365 custom domains are supported, so the API does not restrict the email domain.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/RegisterRequest" },
              "example": {
                "username": "m365.user",
                "email": "user@company.com",
                "phone_number": "+15551234567",
                "password": "password123"
              }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/auth/login": {
      "post": {
        "tags": ["Authentication"],
        "summary": "Log in with email and password",
        "description": "Returns a JWT. If face_required is true, use the token to call POST /api/face/register before accessing protected app endpoints.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/LoginRequest" },
              "example": {
                "email": "demo@example.com",
                "password": "password123"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "JWT token and user profile",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/LoginResponse" }
              }
            }
          },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/auth/forgot-password": {
      "post": {
        "tags": ["Authentication"],
        "summary": "Send password reset code by SMS and email",
        "description": "Creates a short-lived reset code and delivers it through AWS SNS when a phone number is registered, and SMTP email when configured.",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/ForgotPasswordRequest" },
              "example": { "email": "demo@example.com" }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "503": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/auth/reset-password": {
      "post": {
        "tags": ["Authentication"],
        "summary": "Reset password with delivered code",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/ResetPasswordRequest" },
              "example": {
                "email": "demo@example.com",
                "code": "123456",
                "new_password": "newpassword123"
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/auth/face-login": {
      "post": {
        "tags": ["Face Recognition", "Authentication"],
        "summary": "Log in using a face image",
        "description": "Accepts multipart face image uploads. The backend accepts common field names including face, image, file, and photo.",
        "requestBody": { "$ref": "#/components/requestBodies/FaceImageUpload" },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "503": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/upload-face": {
      "post": {
        "tags": ["Face Recognition", "Authentication"],
        "summary": "Alias for face login",
        "requestBody": { "$ref": "#/components/requestBodies/FaceImageUpload" },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "503": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/profile": {
      "get": {
        "tags": ["Profile"],
        "summary": "Get the current user's profile",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/auth/password": {
      "patch": {
        "tags": ["Authentication"],
        "summary": "Update the current user's password",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/UpdatePasswordRequest" },
              "example": {
                "current_password": "password123",
                "new_password": "newpassword123"
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/face/register": {
      "post": {
        "tags": ["Face Recognition"],
        "summary": "Register the current user's face",
        "description": "Required after first password login. This completes onboarding and unlocks protected app endpoints. If AWS Rekognition is not configured, the backend saves the uploaded face locally and completes onboarding in local mode.",
        "security": [{ "BearerAuth": [] }],
        "requestBody": { "$ref": "#/components/requestBodies/FaceImageUpload" },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "409": { "$ref": "#/components/responses/Error" },
          "503": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/face/update": {
      "put": {
        "tags": ["Face Recognition"],
        "summary": "Replace the current user's registered face",
        "security": [{ "BearerAuth": [] }],
        "requestBody": { "$ref": "#/components/requestBodies/FaceImageUpload" },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "503": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/face/delete": {
      "delete": {
        "tags": ["Face Recognition"],
        "summary": "Delete the current user's registered face",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "503": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/helpdesk": {
      "post": {
        "tags": ["Helpdesk"],
        "summary": "Submit a helpdesk ticket",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/HelpdeskTicketRequest" },
              "example": {
                "subject": "Login issue",
                "description": "I cannot access my account."
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/helpdesk-chat": {
      "post": {
        "tags": ["Helpdesk", "AI"],
        "summary": "Send a message to the helpdesk AI assistant",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/HelpdeskChatRequest" },
              "example": { "userMessage": "How do I reset my password?" }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/events": {
      "get": {
        "tags": ["Events"],
        "summary": "List events visible to the current user",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/events/{event_id}/join": {
      "post": {
        "tags": ["Events"],
        "summary": "Join an event chat",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/EventID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "409": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/events/{event_id}/messages": {
      "get": {
        "tags": ["Events"],
        "summary": "List event chat messages",
        "security": [{ "BearerAuth": [] }],
        "parameters": [
          { "$ref": "#/components/parameters/EventID" },
          {
            "name": "limit",
            "in": "query",
            "required": false,
            "schema": { "type": "integer", "default": 50 }
          }
        ],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      },
      "post": {
        "tags": ["Events"],
        "summary": "Send an event chat message",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/EventID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/EventMessageRequest" },
              "example": { "message": "Hello everyone." }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "409": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/events/{event_id}/comments": {
      "get": {
        "tags": ["Events"],
        "summary": "List comments under an event image",
        "description": "Alias for event messages. Responses include comments/messages with a nested sender user object for feed-style display.",
        "security": [{ "BearerAuth": [] }],
        "parameters": [
          { "$ref": "#/components/parameters/EventID" },
          {
            "name": "limit",
            "in": "query",
            "required": false,
            "schema": { "type": "integer", "default": 50 }
          }
        ],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      },
      "post": {
        "tags": ["Events"],
        "summary": "Add a comment under an event image",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/EventID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/EventMessageRequest" },
              "example": { "message": "Great update." }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "409": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/bootstrap": {
      "post": {
        "tags": ["Admin"],
        "summary": "Promote the first authenticated user to admin",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "409": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/users": {
      "get": {
        "tags": ["Admin"],
        "summary": "List all users as an admin",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      },
      "post": {
        "tags": ["Admin"],
        "summary": "Create a user as an admin",
        "description": "Creates a local user account and initializes the user's face-registration profile.",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/AdminCreateUserRequest" },
              "example": {
                "username": "new.user",
                "email": "new.user@example.com",
                "phone_number": "+15551234567",
                "password": "password123",
                "role": "user"
              }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/users/{user_id}": {
      "get": {
        "tags": ["Admin"],
        "summary": "Get a user as an admin",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/UserID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      },
      "patch": {
        "tags": ["Admin"],
        "summary": "Update a user as an admin",
        "description": "Partial update. Send only the fields that should change. Role updates return the full user object instead of only an ID.",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/UserID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/AdminUpdateUserRequest" },
              "example": {
                "username": "updated.user",
                "email": "updated.user@example.com",
                "phone_number": "+15551234567",
                "role": "admin"
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      },
      "delete": {
        "tags": ["Admin"],
        "summary": "Delete a user as an admin",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/UserID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/events": {
      "post": {
        "tags": ["Admin"],
        "summary": "Create an event as an admin",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/EventCreateRequest" },
              "example": {
                "title": "Team briefing",
                "description": "Daily operations update",
                "image_url": "https://example.com/event-image.jpg"
              }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/events/{event_id}": {
      "patch": {
        "tags": ["Admin"],
        "summary": "Update an event as an admin",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/EventID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/EventUpdateRequest" },
              "example": {
                "title": "Updated briefing",
                "description": "Updated operations update",
                "image_url": "https://example.com/event-image.jpg",
                "status": "active"
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      },
      "delete": {
        "tags": ["Admin"],
        "summary": "Delete an event as an admin",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/EventID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/users/{user_id}/role": {
      "patch": {
        "tags": ["Admin"],
        "summary": "Update a user's role",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/UserID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/UserRoleRequest" },
              "example": { "role": "admin" }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/events/{event_id}/messages": {
      "get": {
        "tags": ["Admin", "Events"],
        "summary": "Admin list event chat messages",
        "security": [{ "BearerAuth": [] }],
        "parameters": [
          { "$ref": "#/components/parameters/EventID" },
          {
            "name": "limit",
            "in": "query",
            "required": false,
            "schema": { "type": "integer", "default": 50 }
          }
        ],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      },
      "post": {
        "tags": ["Admin", "Events"],
        "summary": "Admin send an event chat message",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/EventID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/EventMessageRequest" },
              "example": { "message": "Welcome to the event." }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "409": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/helpdesk": {
      "get": {
        "tags": ["Admin", "Helpdesk"],
        "summary": "List helpdesk tickets as an admin",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/helpdesk/{ticket_id}": {
      "get": {
        "tags": ["Admin", "Helpdesk"],
        "summary": "Get a helpdesk ticket as an admin",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/TicketID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      },
      "patch": {
        "tags": ["Admin", "Helpdesk"],
        "summary": "Update a helpdesk ticket as an admin",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/TicketID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/HelpdeskUpdateRequest" },
              "example": {
                "subject": "Login issue",
                "description": "Customer cannot access the app.",
                "status": "in_progress"
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" }
        }
      },
      "delete": {
        "tags": ["Admin", "Helpdesk"],
        "summary": "Delete a helpdesk ticket as an admin",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/TicketID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/chatbot": {
      "post": {
        "tags": ["AI"],
        "summary": "Send a message to the OpenAI web-search analytics agent",
        "description": "Uses OpenAI Responses API. Web search is enabled by default for current public information, website research, and analytics-style questions. Research answers are instructed to use multiple independent sources when available.",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/ChatbotRequest" },
              "example": {
                "message": "Analyze https://example.com and tell me improvements with sources.",
                "web_search": true
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Chatbot response",
            "content": {
              "application/json": {
                "schema": { "$ref": "#/components/schemas/ChatbotResponse" }
              }
            }
          },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "503": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/bpo/analyze-pdf": {
      "post": {
        "tags": ["BPO Analysis"],
        "summary": "Upload a PDF for asynchronous BPO analysis",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "multipart/form-data": {
              "schema": {
                "type": "object",
                "required": ["document"],
                "properties": {
                  "document": {
                    "type": "string",
                    "format": "binary",
                    "description": "PDF document, maximum 20 MB"
                  }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/bpo/analysis/{id}": {
      "get": {
        "tags": ["BPO Analysis"],
        "summary": "Get a BPO analysis by ID",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/AnalysisID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      },
      "delete": {
        "tags": ["BPO Analysis"],
        "summary": "Delete a BPO analysis by ID",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/AnalysisID" }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "404": { "$ref": "#/components/responses/Error" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/bpo/analyses": {
      "get": {
        "tags": ["BPO Analysis"],
        "summary": "List BPO analyses",
        "security": [{ "BearerAuth": [] }],
        "parameters": [
          {
            "name": "page",
            "in": "query",
            "schema": { "type": "integer", "default": 1, "minimum": 1 }
          },
          {
            "name": "limit",
            "in": "query",
            "schema": { "type": "integer", "default": 10, "minimum": 1, "maximum": 100 }
          },
          {
            "name": "status",
            "in": "query",
            "schema": { "type": "string" }
          },
          {
            "name": "type",
            "in": "query",
            "schema": { "type": "string" }
          }
        ],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/users": {
      "get": {
        "tags": ["Users", "Direct Chat"],
        "summary": "List users available for direct chat",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/chats": {
      "get": {
        "tags": ["Direct Chat"],
        "summary": "List direct chat conversations for the current user",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/chats/{user_id}/messages": {
      "get": {
        "tags": ["Direct Chat"],
        "summary": "List direct messages with another user",
        "security": [{ "BearerAuth": [] }],
        "parameters": [
          { "$ref": "#/components/parameters/UserID" },
          {
            "name": "limit",
            "in": "query",
            "schema": { "type": "integer", "default": 50, "minimum": 1, "maximum": 100 }
          }
        ],
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      },
      "post": {
        "tags": ["Direct Chat"],
        "summary": "Send a direct message to another user",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/UserID" }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/DirectMessageRequest" },
              "example": { "message": "Hello, can we chat?" }
            }
          }
        },
        "responses": {
          "201": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/cv": {
      "get": {
        "tags": ["CV Builder"],
        "summary": "Get the current user's CV data",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": {
            "description": "CV profile",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": { "cv": { "$ref": "#/components/schemas/CVProfile" } }
                }
              }
            }
          },
          "401": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" }
        }
      },
      "post": {
        "tags": ["CV Builder"],
        "summary": "Create or fully replace the current user's CV data",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/CVUpsertRequest" },
              "example": {
                "first_name": "Jane",
                "last_name": "Doe",
                "profile_text": "Experienced BPO analyst with 8 years in process optimisation.",
                "value_proposition": "I streamline back-office operations to reduce cost by 20-30%.",
                "gender": "Female",
                "nationality": "South African",
                "date_of_birth": "1990-06-15",
                "professional_skills": [
                  { "skill": "Process Management", "details": ["Lean Six Sigma", "BPMN modelling"] }
                ],
                "qualifications": ["BCom Information Systems, UCT, 2012"],
                "computer_skills": ["Microsoft Office 365", "SAP", "Power BI"],
                "professional_memberships": ["IITPSA"],
                "languages": ["English", "Afrikaans"],
                "experience": [
                  {
                    "company": "Acme BPO",
                    "position": "Senior Analyst",
                    "period_start": "2018-03",
                    "period_end": "2024-01",
                    "scope_of_work": ["Led a team of 5 analysts", "Reduced processing time by 25%"]
                  }
                ]
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Saved CV",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": { "cv": { "$ref": "#/components/schemas/CVProfile" } }
                }
              }
            }
          },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      },
      "patch": {
        "tags": ["CV Builder"],
        "summary": "Partially update the current user's CV data",
        "description": "Same handler as POST — send only the fields you want to change. Omitted fields are kept as-is.",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": { "$ref": "#/components/schemas/CVUpsertRequest" },
              "example": {
                "profile_text": "Updated profile summary."
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/cv/photo": {
      "post": {
        "tags": ["CV Builder"],
        "summary": "Upload a profile photo for the CV",
        "description": "Accepts JPG or PNG up to 5 MB. The photo is stored in S3 and the URL is saved to the CV record.",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "multipart/form-data": {
              "schema": {
                "type": "object",
                "required": ["photo"],
                "properties": {
                  "photo": {
                    "type": "string",
                    "format": "binary",
                    "description": "JPG or PNG image, max 5 MB"
                  }
                }
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Photo uploaded",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": { "profile_photo_url": { "type": "string", "format": "uri" } }
                }
              }
            }
          },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/cv/download": {
      "get": {
        "tags": ["CV Builder"],
        "summary": "Download the current user's CV as a branded PDF",
        "description": "Generates a two-page SoluGrowth-branded PDF from the stored CV data and returns it as a file download.",
        "security": [{ "BearerAuth": [] }],
        "responses": {
          "200": {
            "description": "PDF file",
            "content": {
              "application/pdf": {
                "schema": { "type": "string", "format": "binary" }
              }
            }
          },
          "401": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/cv/search": {
      "get": {
        "tags": ["CV Builder"],
        "summary": "Search the talent directory by skill or qualification",
        "description": "At least one of 'skill' or 'qualification' must be provided. Returns matching CV summaries including each person's skills and qualifications. Case-insensitive substring match.",
        "security": [{ "BearerAuth": [] }],
        "parameters": [
          {
            "name": "skill",
            "in": "query",
            "required": false,
            "description": "Substring to match against professional skills (e.g. 'Python', 'SAP', 'Lean')",
            "schema": { "type": "string", "example": "SAP" }
          },
          {
            "name": "qualification",
            "in": "query",
            "required": false,
            "description": "Substring to match against qualifications (e.g. 'BCom', 'MBA', 'PMP')",
            "schema": { "type": "string", "example": "BCom" }
          }
        ],
        "responses": {
          "200": {
            "description": "Matching CV summaries",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "results": {
                      "type": "array",
                      "items": { "$ref": "#/components/schemas/CVProfileSummary" }
                    },
                    "count": { "type": "integer" }
                  }
                }
              }
            }
          },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/cvs": {
      "get": {
        "tags": ["CV Builder", "Admin"],
        "summary": "List all CV profiles (admin) with optional filters",
        "description": "Returns all CVs by default. Use 'skill' or 'qualification' query params to narrow results. Case-insensitive substring match on JSON data.",
        "security": [{ "BearerAuth": [] }],
        "parameters": [
          {
            "name": "skill",
            "in": "query",
            "required": false,
            "description": "Filter by skill substring (e.g. 'Python', 'SAP')",
            "schema": { "type": "string", "example": "SAP" }
          },
          {
            "name": "qualification",
            "in": "query",
            "required": false,
            "description": "Filter by qualification substring (e.g. 'BCom', 'MBA')",
            "schema": { "type": "string", "example": "BCom" }
          }
        ],
        "responses": {
          "200": {
            "description": "CV summaries",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "cvs": {
                      "type": "array",
                      "items": { "$ref": "#/components/schemas/CVProfileSummary" }
                    },
                    "count": { "type": "integer" }
                  }
                }
              }
            }
          },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/admin/cvs/{user_id}": {
      "get": {
        "tags": ["CV Builder", "Admin"],
        "summary": "Get a specific user's full CV (admin)",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/UserID" }],
        "responses": {
          "200": {
            "description": "Full CV profile",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": { "cv": { "$ref": "#/components/schemas/CVProfile" } }
                }
              }
            }
          },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" }
        }
      }
    },
    "/api/admin/cvs/{user_id}/download": {
      "get": {
        "tags": ["CV Builder", "Admin"],
        "summary": "Download any user's CV as a branded PDF (admin)",
        "security": [{ "BearerAuth": [] }],
        "parameters": [{ "$ref": "#/components/parameters/UserID" }],
        "responses": {
          "200": {
            "description": "PDF file",
            "content": {
              "application/pdf": {
                "schema": { "type": "string", "format": "binary" }
              }
            }
          },
          "401": { "$ref": "#/components/responses/Error" },
          "403": { "$ref": "#/components/responses/Error" },
          "404": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    },
    "/api/upload": {
      "post": {
        "tags": ["Uploads"],
        "summary": "Upload a general file",
        "security": [{ "BearerAuth": [] }],
        "requestBody": {
          "required": true,
          "content": {
            "multipart/form-data": {
              "schema": {
                "type": "object",
                "required": ["file"],
                "properties": {
                  "file": {
                    "type": "string",
                    "format": "binary",
                    "description": "File upload, maximum 10 MB"
                  }
                }
              }
            }
          }
        },
        "responses": {
          "200": { "$ref": "#/components/responses/Success" },
          "400": { "$ref": "#/components/responses/Error" },
          "401": { "$ref": "#/components/responses/Error" },
          "428": { "$ref": "#/components/responses/FaceRegistrationRequired" },
          "500": { "$ref": "#/components/responses/Error" }
        }
      }
    }
  },
  "components": {
    "securitySchemes": {
      "BearerAuth": {
        "type": "http",
        "scheme": "bearer",
        "bearerFormat": "JWT"
      }
    },
    "parameters": {
      "EventID": {
        "name": "event_id",
        "in": "path",
        "required": true,
        "schema": { "type": "integer", "minimum": 1 }
      },
      "UserID": {
        "name": "user_id",
        "in": "path",
        "required": true,
        "schema": { "type": "integer", "minimum": 1 }
      },
      "TicketID": {
        "name": "ticket_id",
        "in": "path",
        "required": true,
        "schema": { "type": "integer", "minimum": 1 }
      },
      "AnalysisID": {
        "name": "id",
        "in": "path",
        "required": true,
        "schema": { "type": "string" }
      }
    },
    "requestBodies": {
      "FaceImageUpload": {
        "required": true,
        "content": {
          "multipart/form-data": {
            "schema": {
              "type": "object",
              "required": ["face"],
              "properties": {
                "face": {
                  "type": "string",
                  "format": "binary",
                  "description": "JPEG or PNG face image, maximum 5 MB"
                }
              }
            }
          }
        }
      }
    },
    "responses": {
      "Success": {
        "description": "Successful response",
        "content": {
          "application/json": {
            "schema": { "type": "object", "additionalProperties": true }
          }
        }
      },
      "Error": {
        "description": "Error response",
        "content": {
          "application/json": {
            "schema": { "$ref": "#/components/schemas/ErrorResponse" }
          }
        }
      },
      "FaceRegistrationRequired": {
        "description": "The authenticated user must complete face registration first.",
        "content": {
          "application/json": {
            "schema": { "$ref": "#/components/schemas/FaceRegistrationRequiredResponse" },
            "example": {
              "error": "Face registration required",
              "code": "FACE_REGISTRATION_REQUIRED",
              "message": "Register your face before using this endpoint.",
              "face_required": true,
              "face_status": false,
              "next_step": "POST /api/face/register"
            }
          }
        }
      }
    },
    "schemas": {
      "RegisterRequest": {
        "type": "object",
        "required": ["username", "email", "password"],
        "properties": {
          "username": { "type": "string" },
          "email": { "type": "string", "format": "email" },
          "phone_number": { "type": "string", "description": "E.164 format is recommended for AWS SNS SMS delivery." },
          "password": { "type": "string", "minLength": 6 }
        }
      },
      "LoginRequest": {
        "type": "object",
        "required": ["email", "password"],
        "properties": {
          "email": { "type": "string", "format": "email" },
          "password": { "type": "string" }
        }
      },
      "ForgotPasswordRequest": {
        "type": "object",
        "required": ["email"],
        "properties": {
          "email": { "type": "string", "format": "email" }
        }
      },
      "ResetPasswordRequest": {
        "type": "object",
        "required": ["email", "code", "new_password"],
        "properties": {
          "email": { "type": "string", "format": "email" },
          "code": { "type": "string", "minLength": 4 },
          "new_password": { "type": "string", "minLength": 6 }
        }
      },
      "UpdatePasswordRequest": {
        "type": "object",
        "required": ["current_password", "new_password"],
        "properties": {
          "current_password": { "type": "string" },
          "new_password": { "type": "string", "minLength": 6 }
        }
      },
      "LoginResponse": {
        "type": "object",
        "properties": {
          "message": { "type": "string" },
          "token": { "type": "string" },
          "face_required": { "type": "boolean" },
          "next_step": { "type": "string" },
          "user": { "$ref": "#/components/schemas/User" }
        }
      },
      "User": {
        "type": "object",
        "properties": {
          "id": { "type": "integer" },
          "username": { "type": "string" },
          "email": { "type": "string", "format": "email" },
          "phone_number": { "type": "string" },
          "auth_provider": { "type": "string", "enum": ["local", "outlook365"] },
          "role": { "type": "string", "enum": ["user", "admin"] },
          "face_status": { "type": "boolean" },
          "image_url": { "type": "string", "nullable": true }
        }
      },
      "HelpdeskTicketRequest": {
        "type": "object",
        "properties": {
          "subject": { "type": "string" },
          "description": { "type": "string" }
        }
      },
      "HelpdeskUpdateRequest": {
        "type": "object",
        "properties": {
          "subject": { "type": "string" },
          "description": { "type": "string" },
          "status": { "type": "string", "enum": ["open", "in_progress", "resolved", "closed"] }
        }
      },
      "HelpdeskChatRequest": {
        "type": "object",
        "required": ["userMessage"],
        "properties": {
          "userMessage": { "type": "string" }
        }
      },
      "EventCreateRequest": {
        "type": "object",
        "required": ["title"],
        "properties": {
          "title": { "type": "string", "maxLength": 255 },
          "description": { "type": "string" },
          "image_url": { "type": "string", "format": "uri", "description": "Image displayed above comments in the event feed." }
        }
      },
      "EventUpdateRequest": {
        "type": "object",
        "properties": {
          "title": { "type": "string", "maxLength": 255 },
          "description": { "type": "string" },
          "image_url": { "type": "string", "format": "uri" },
          "status": { "type": "string", "enum": ["active", "closed"] }
        }
      },
      "AdminCreateUserRequest": {
        "type": "object",
        "required": ["username", "email", "password"],
        "properties": {
          "username": { "type": "string" },
          "email": { "type": "string", "format": "email" },
          "phone_number": { "type": "string" },
          "password": { "type": "string", "minLength": 6 },
          "role": { "type": "string", "enum": ["user", "admin"], "default": "user" }
        }
      },
      "AdminUpdateUserRequest": {
        "type": "object",
        "properties": {
          "username": { "type": "string" },
          "email": { "type": "string", "format": "email" },
          "phone_number": { "type": "string" },
          "password": { "type": "string", "minLength": 6 },
          "role": { "type": "string", "enum": ["user", "admin"] }
        }
      },
      "EventMessageRequest": {
        "type": "object",
        "required": ["message"],
        "properties": {
          "message": { "type": "string", "maxLength": 4000 }
        }
      },
      "DirectMessageRequest": {
        "type": "object",
        "required": ["message"],
        "properties": {
          "message": { "type": "string", "maxLength": 4000 }
        }
      },
      "UserRoleRequest": {
        "type": "object",
        "required": ["role"],
        "properties": {
          "role": { "type": "string", "enum": ["user", "admin"] }
        }
      },
      "ChatbotRequest": {
        "type": "object",
        "required": ["message"],
        "properties": {
          "message": { "type": "string" },
          "web_search": {
            "type": "boolean",
            "description": "Defaults to true. Disable only when you want no public web lookup."
          }
        }
      },
      "ChatbotResponse": {
        "type": "object",
        "properties": {
          "reply": { "type": "string" },
          "sources": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "title": { "type": "string" },
                "url": { "type": "string", "format": "uri" }
              }
            }
          },
          "source_count": { "type": "integer" },
          "model": { "type": "string" },
          "web_search_enabled": { "type": "boolean" },
          "error": { "type": "string" }
        }
      },
      "HealthResponse": {
        "type": "object",
        "properties": {
          "message": { "type": "string" },
          "timestamp": { "type": "string", "format": "date-time" },
          "database": { "type": "string" },
          "database_type": { "type": "string" },
          "database_host": { "type": "string" },
          "database_name": { "type": "string" },
          "aws_status": { "type": "string" },
          "openai_status": { "type": "string" },
          "openai_model": { "type": "string" },
          "secrets_source": { "type": "string" },
          "secret_name": { "type": "string" },
          "port": { "type": "integer" }
        }
      },
      "ErrorResponse": {
        "type": "object",
        "properties": {
          "error": { "type": "string" },
          "message": { "type": "string" },
          "details": { "type": "string" }
        }
      },
      "CVUpsertRequest": {
        "type": "object",
        "properties": {
          "first_name": { "type": "string" },
          "last_name": { "type": "string" },
          "profile_text": { "type": "string", "description": "Max ~80 words" },
          "value_proposition": { "type": "string", "description": "Max ~150 words" },
          "gender": { "type": "string" },
          "nationality": { "type": "string" },
          "date_of_birth": { "type": "string", "format": "date", "description": "YYYY-MM-DD" },
          "professional_skills": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/ProfessionalSkill" }
          },
          "qualifications": { "type": "array", "items": { "type": "string" } },
          "computer_skills": { "type": "array", "items": { "type": "string" } },
          "professional_memberships": { "type": "array", "items": { "type": "string" } },
          "languages": { "type": "array", "items": { "type": "string" } },
          "experience": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/CVExperience" }
          }
        }
      },
      "CVProfile": {
        "allOf": [
          { "$ref": "#/components/schemas/CVUpsertRequest" },
          {
            "type": "object",
            "properties": {
              "id": { "type": "integer" },
              "user_id": { "type": "integer" },
              "profile_photo_url": { "type": "string", "format": "uri" },
              "created_at": { "type": "string", "format": "date-time" },
              "updated_at": { "type": "string", "format": "date-time" }
            }
          }
        ]
      },
      "CVProfileSummary": {
        "type": "object",
        "properties": {
          "user_id": { "type": "integer" },
          "first_name": { "type": "string" },
          "last_name": { "type": "string" },
          "professional_skills": {
            "type": "array",
            "items": { "$ref": "#/components/schemas/ProfessionalSkill" }
          },
          "qualifications": { "type": "array", "items": { "type": "string" } },
          "updated_at": { "type": "string", "format": "date-time" }
        }
      },
      "ProfessionalSkill": {
        "type": "object",
        "properties": {
          "skill": { "type": "string", "example": "Project Management" },
          "details": { "type": "array", "items": { "type": "string" }, "example": ["PMP certified", "Agile"] }
        }
      },
      "CVExperience": {
        "type": "object",
        "properties": {
          "company": { "type": "string" },
          "position": { "type": "string" },
          "period_start": { "type": "string", "description": "YYYY-MM", "example": "2018-03" },
          "period_end": { "type": "string", "description": "YYYY-MM or empty for present", "example": "2024-01" },
          "scope_of_work": { "type": "array", "items": { "type": "string" } }
        }
      },
      "FaceRegistrationRequiredResponse": {
        "type": "object",
        "properties": {
          "error": { "type": "string" },
          "code": { "type": "string", "example": "FACE_REGISTRATION_REQUIRED" },
          "message": { "type": "string" },
          "face_required": { "type": "boolean" },
          "face_status": { "type": "boolean" },
          "next_step": { "type": "string", "example": "POST /api/face/register" }
        }
      }
    }
  }
}`
