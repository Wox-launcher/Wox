# -*- coding: utf-8 -*-

import json
import subprocess
import webbrowser

from wox import Wox
from wox import WoxAPI

URL = {
    'Google': ('search on google', 'https://google.com'),
    'Github': ('search open source repositories', 'https://github.com'),
    'Reddit': ('Be unproductive', 'https://reddit.com'),
}

class HelloWorld(Wox):
    """Example python plugin.

    Plugins must implement two method:
    - query(self, arg)
    - context_menu(self, arg)
    They must return a list of results.

    Other functions should not have return value.
    """
    def query(self, arg):
        """Function that is invoked when plugin is selected.

        The user can pass a query string, arg, that this function can use as input.
        This can be used to filter results or pass as input to the actions from results.
        
        In this example, the query will search for substring in title and subtitle.
        """
        def result(title, subtitle, url):
            return {
                'Title': title,
                'SubTitle': 'Subtitle: {}'.format(subtitle),
                'IcoPath': 'Images/app.png',
                'ContextData': title,  # Data that is passed to context_menu function call
                'JsonRPCAction': {
                    'method': 'open_webpage',  # Maps to function name that should be called if this action is selected.
                    'parameters': [url, False],  # N number of params that will be passed to function call
                }
            }

        search_term = arg.lower()

        def search_criteria(key, value):
            key = key.lower()
            description = value[0].lower()

            return search_term in key or search_term in description
        return [result(key, *value) for key, value in URL.items() if search_criteria(key, value)]

    def context_menu(self, ctx_data):
        """Function that is called when context menu is triggered (shift-enter).

        ctx_data is the value set in from ContextData from query.
        
        Note: The user's query in context menu is only for searching through the title of the following results.
        """
        site_title = ctx_data
        results = [
            {
                "Title": "Open up {} in a new window".format(site_title),
                'JsonRPCAction': {
                    'method': 'open_webpage',
                    'parameters': [URL[site_title][1]],
                },
            },
            {
                "Title": "Show notification",
                'JsonRPCAction': {
                    'method': 'show_notification',
                    'parameters': [site_title, 'some addiitional text'],
                },
            },
            {
                "Title": "Kick off whatever process you want",
                'JsonRPCAction': {
                    'method': 'run_process',
                    'parameters': [],
                },
            },
        ]
        return results

    def open_webpage(self, url, notification=True):
        """Trivial implementation of bookmarks in Wox

        Demonstrates how optional parameters in Json RPC are handled.
        """
        if notification:
            WoxAPI.show_msg('Heads up', 'We opened in new window')
            webbrowser.open_new(url)
        else:
            webbrowser.open(url)

    def show_notification(self, site, extra_message):
        """Custom handler that demonstrates how to call Wox api"""
        title = 'You chose {}'.format(site)
        sub_title = 'This the extra message you sent: {}'.format(extra_message)
        ico_path = ''

        # Wrappers for many common APIs are found in JsonRPC/wox.py
        # WoxAPI.show_msg(title, sub_title)

        # But you can invoke any arbitrary function in Wox dlls with JSON RPC
        print(json.dumps(
            {
                "method": "Wox.ShowMsg",
                "parameters": [ title, sub_title, ico_path ]
            },
        ))

    def run_process(self):
        """Custom handler to trigger any process to run."""
        subprocess.call(['/bin/true'])


if __name__ == "__main__":
    HelloWorld()
