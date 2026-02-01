# HattieBot Tools

For a comprehensive list of available tools, please refer to the **Built-in Tools** section in [architecture.md](./architecture.md).

## Tool Registry

Agents can extend the system by creating new tools.

1. **Create Source**: Write Go code in `$CONFIG_DIR/tools/<toolname>/main.go`.
2. **Build**: Compile to `$CONFIG_DIR/bin/<toolname>`.
3. **Register**: Use the `register_tool` function to add it to the database.

Registered tools persist across restarts and updates.
