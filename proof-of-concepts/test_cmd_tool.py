#!/usr/bin/env python3
"""Test the two-turn cmd capability flow without executing commands."""

import json
import subprocess
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

API_URL = "https://openrouter.ai/api/v1/chat/completions"
USER_REQUEST = (
    "Create two different files: alpha.txt containing Alpha and beta.txt containing Beta."
)

SECOND_TURN_SYSTEM_PROMPT = """You are an agent working inside a project.

Available terminal CLI tools:

User-defined tools:
- edit --oldtext TEXT --newtext TEXT: edit an existing file
- read FILE: read a file
- write FILE --content TEXT: create or overwrite a file

Common tools:
- ls: list files
- cd DIRECTORY: change directory
- mkdir DIRECTORY: create a directory

Use the available CLI tools to complete the user's request."""


def get_api_key() -> str:
    try:
        key = subprocess.run(
            ["pass", "show", "pi/openrouter"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout.strip()
    except FileNotFoundError as error:
        raise RuntimeError("pass is not installed") from error
    except subprocess.CalledProcessError as error:
        raise RuntimeError("could not read pass entry pi/openrouter") from error
    if not key:
        raise RuntimeError("pass entry pi/openrouter is empty")
    return key


def call_openrouter(api_key: str, messages: list[dict], tools: list[dict]) -> dict:
    payload = {
        "model": "@preset/mimo",
        "messages": messages,
        "tools": tools,
        "tool_choice": "auto",
    }
    request = Request(
        API_URL,
        data=json.dumps(payload).encode(),
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
            "HTTP-Referer": "https://github.com/",
            "X-Title": "gq cmd proof of concept",
        },
        method="POST",
    )
    try:
        with urlopen(request, timeout=120) as response:
            return json.load(response)
    except HTTPError as error:
        body = error.read().decode(errors="replace")
        raise RuntimeError(f"OpenRouter returned HTTP {error.code}: {body}") from error
    except URLError as error:
        raise RuntimeError(f"network error: {error.reason}") from error


def argumentless_do_tool() -> dict:
    return {
        "type": "function",
        "function": {
            "name": "do",
            "description": "Call this tool with no arguments to execute an action.",
            "parameters": {
                "type": "object",
                "properties": {},
                "additionalProperties": False,
            },
        },
    }


def action_cmd_tool() -> dict:
    return {
        "type": "function",
        "function": {
            "name": "cmd",
            "description": "Execute a terminal CLI action.",
            "parameters": {
                "type": "object",
                "properties": {
                    "cmd": {"type": "string"}
                },
                "required": ["cmd"],
                "additionalProperties": False,
            },
        },
    }


def main() -> int:
    try:
        api_key = get_api_key()

        # Turn 1: only the argumentless do capability is visible.
        first = call_openrouter(
            api_key,
            [
                {
                    "role": "system",
                    "content": "You are an agent that can make changes to a project. When an action is needed, use the available tool.",
                },
                {"role": "user", "content": USER_REQUEST},
            ],
            [argumentless_do_tool()],
        )
        first_choice = first["choices"][0]
        first_message = first_choice["message"]
        first_calls = first_message.get("tool_calls") or []
        first_called_cmd = any(
            call.get("function", {}).get("name") == "do" for call in first_calls
        )
        print("PASS: first turn called do" if first_called_cmd else "FAIL: first turn did not call do")
        print("\nFIRST TURN RESPONSE:")
        print(json.dumps(first_message, indent=2))

        # Turn 2: reconstruct the context. The first do call is not included.
        second = call_openrouter(
            api_key,
            [
                {"role": "system", "content": SECOND_TURN_SYSTEM_PROMPT},
                {"role": "user", "content": USER_REQUEST},
            ],
            [action_cmd_tool()],
        )
        second_choice = second["choices"][0]
        second_message = second_choice["message"]
        second_calls = second_message.get("tool_calls") or []
        second_called_cmd = any(
            call.get("function", {}).get("name") == "cmd" for call in second_calls
        )
        print("\nSECOND TURN RESPONSE:")
        print(json.dumps(second_message, indent=2))
        print("\nPASS: second turn called cmd" if second_called_cmd else "\nFAIL: second turn did not call cmd")

        # Continue the agent loop. After each hidden cmd action, the model sees
        # only do() and the semantic result; it never sees cmd or its arguments.
        action_result = "Created alpha.txt. beta.txt has not been created yet."
        action_number = 1
        while True:
            decision = call_openrouter(
                api_key,
                [
                    {
                        "role": "system",
                        "content": "Decide whether the user's request is complete based on the result from do. If it is complete, respond without a tool call. If another action is needed, call do with no arguments.",
                    },
                    {"role": "user", "content": USER_REQUEST},
                    {
                        "role": "assistant",
                        "tool_calls": [
                            {
                                "id": f"do-call-{action_number}",
                                "type": "function",
                                "function": {"name": "do", "arguments": "{}"},
                            }
                        ],
                    },
                    {
                        "role": "tool",
                        "tool_call_id": f"do-call-{action_number}",
                        "content": action_result,
                    },
                ],
                [argumentless_do_tool()],
            )
            decision_message = decision["choices"][0]["message"]
            decision_calls = decision_message.get("tool_calls") or []
            wants_next_turn = any(
                call.get("function", {}).get("name") == "do"
                for call in decision_calls
            )
            print(f"\nDECISION TURN {action_number + 2} RESPONSE:")
            print(json.dumps(decision_message, indent=2))

            if not wants_next_turn:
                print("\nPASS: agent loop closed after semantic completion")
                break

            action_number += 1
            if action_number > 2:
                print("\nFAIL: agent loop exceeded the two-action simulation")
                break

            # The hidden executor would run the second action here. This is
            # simulated so the proof of concept never touches the filesystem.
            action_result = "Created beta.txt. Both requested files now exist."
            next_action = call_openrouter(
                api_key,
                [
                    {"role": "system", "content": SECOND_TURN_SYSTEM_PROMPT},
                    {"role": "user", "content": USER_REQUEST},
                ],
                [action_cmd_tool()],
            )
            next_message = next_action["choices"][0]["message"]
            print(f"\nHIDDEN ACTION TURN {action_number + 2} RESPONSE:")
            print(json.dumps(next_message, indent=2))

        print("NOTE: no command was executed.")
        return 0
    except (KeyError, IndexError, TypeError, json.JSONDecodeError) as error:
        print(f"invalid model response: {error}", file=sys.stderr)
        return 1
    except RuntimeError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
