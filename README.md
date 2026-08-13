# wire-pod

## Overview

wire-pod is a third-party MCP server implementation for the Anki Vector robot, providing an alternative interface to control and interact with Vector. It enables HTTP-based communication with the robot, allowing for integration with web services, custom voice commands, and AI assistants.

## Features

- RESTful API for controlling Vector
- WebSocket support for real-time communication
- Custom wake word detection
- Integration with various LLMs and AI platforms
- Support for multiple concurrent clients
- Extensible plugin system

## Setup

1. Install wire-pod server on a compatible device
2. Connect to the same network as your Vector robot
3. Pair with Vector using the setup code
4. Configure desired integrations and plugins

## Usage

The wire-pod server exposes endpoints for:

- Voice interaction
- Robot movement and navigation
- Animation and LED control
- Sensor data retrieval
- Cube interaction

## Integration with vector-mcp

This project will explore integrating wire-pod as an alternative MCP server source for the vector-mcp project, allowing for:

- Redundant communication channels
- Load balancing between MCP servers
- Feature comparison and selection
- Enhanced reliability through failover capabilities