"""Plugin manager for handling plugin instances and responses"""
from typing import Dict, Any

# Global state
plugin_instances: Dict[str, Dict[str, Any]] = {}
waiting_for_response: Dict[str, Any] = {} 