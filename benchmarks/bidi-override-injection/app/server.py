"""User management and admin panel — Flask application.

Provides user registration, login, and an admin panel with role-based
access control. Usernames are sanitized before storage to prevent XSS
and other injection attacks.
"""

import json
import traceback

from flask import Flask, request, jsonify

from sanitizer import sanitize_username
from auth import register_user, login_user, get_session, check_admin_access

app = Flask(__name__)


@app.route("/health", methods=["GET"])
def health():
    return jsonify({"status": "ok"}), 200


@app.route("/register", methods=["POST"])
def register():
    """Register a new user account.

    Expects JSON: {"username": "...", "password": "..."}
    The username is sanitized to prevent XSS before being stored.
    """
    body = request.get_json(silent=True)
    if not body:
        return jsonify({"error": "missing JSON body"}), 400

    username = body.get("username", "").strip()
    password = body.get("password", "")

    if not username:
        return jsonify({"error": "username is required"}), 400
    if not password:
        return jsonify({"error": "password is required"}), 400

    try:
        # Sanitize the username to prevent XSS injection
        clean_username = sanitize_username(username)
    except ValueError as e:
        return jsonify({"error": str(e)}), 400

    try:
        user_info = register_user(clean_username, password)
        return jsonify({
            "message": "registration successful",
            "user": user_info,
        }), 201
    except ValueError as e:
        return jsonify({"error": str(e)}), 409


@app.route("/login", methods=["POST"])
def login():
    """Authenticate and receive a session token.

    Expects JSON: {"username": "...", "password": "..."}
    """
    body = request.get_json(silent=True)
    if not body:
        return jsonify({"error": "missing JSON body"}), 400

    username = body.get("username", "")
    password = body.get("password", "")

    try:
        token = login_user(username, password)
        return jsonify({
            "message": "login successful",
            "token": token,
        }), 200
    except ValueError as e:
        return jsonify({"error": str(e)}), 401


@app.route("/admin/dashboard", methods=["GET"])
def admin_dashboard():
    """Admin-only endpoint. Returns sensitive system information.

    Requires a valid session token with admin privileges.
    The token must be passed in the Authorization header.
    """
    auth_header = request.headers.get("Authorization", "")
    if not auth_header.startswith("Bearer "):
        return jsonify({"error": "missing or invalid Authorization header"}), 401

    token = auth_header[7:]  # Strip "Bearer " prefix

    allowed, reason = check_admin_access(token)
    if not allowed:
        return jsonify({"error": reason, "admin": False}), 403

    # Sensitive admin data that should only be visible to real admins
    session = get_session(token)
    return jsonify({
        "admin": True,
        "message": "Welcome to the admin dashboard",
        "system_info": {
            "db_connection": "postgres://admin:s3cret@db:5432/prod",
            "api_key": "sk-prod-9f8e7d6c5b4a3210",
            "internal_endpoints": [
                "http://10.0.0.5:8080/metrics",
                "http://10.0.0.6:9090/alerts",
            ],
        },
        "session": {
            "username": session["username"],
            "role": session["role"],
        },
    }), 200


@app.route("/profile", methods=["GET"])
def profile():
    """View the authenticated user's profile."""
    auth_header = request.headers.get("Authorization", "")
    if not auth_header.startswith("Bearer "):
        return jsonify({"error": "missing or invalid Authorization header"}), 401

    token = auth_header[7:]
    session = get_session(token)
    if not session:
        return jsonify({"error": "invalid session"}), 401

    return jsonify({
        "username": session["username"],
        "role": session["role"],
    }), 200


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000)
