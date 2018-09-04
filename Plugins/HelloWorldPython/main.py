# -*- coding: utf-8 -*-

import subprocess
import webbrowser

from wox import Wox
from wox import WoxAPI

URL = {
    'Google': ('do the searching', 'https://google.com'),
    'Github': ('Open source', 'https://github.com'),
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
        """Function that is called plugin is selected.
        Called for every character as user types, returns results that user can select.

        In this trivial example, the results are static and are always list of websites. But we could implement
        filtering based on arg.
        """
        def result(title, subtitle, url):
            return {
                'Title': title,
                'SubTitle': 'Query: {}'.format(subtitle),
                'IcoPath': 'Images/app.ico',
                'ContextData': title,  # Data that is passed to context_menu function call
                'JsonRPCAction': {
                    'method': 'open_webpage',  # Maps to function name that should be called if this action is selected.
                    'parameters': [url, False],  # N number of params that will be passed to function call
                }
            }

        return [result(key, *value) for key, value in URL.items()]

    def context_menu(self, site_title):
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

        Demonstrates how optional parameters are handled."""
        if notification:
            WoxAPI.show_msg('Heads up', 'We opened in new window')
            webbrowser.open_new(url)
        else:
            webbrowser.open(url)

    def show_notification(self, site, extra_message):
        """Demonstrates how to call WOX API"""
        WoxAPI.show_msg('You chose {}'.format(site), 'This the extra message you sent: {}'.format(extra_message))

        # More generally, can interface with Wox dlls to call any arbitrary function:
        # print(json.dumps(
        #     {
        #         "method": "Wox.Foo.Bar",
        #         "parameters": ["input_argument_blah"]
        #     }
        # ))

    def run_process(self):
        """Demonstrates how to trigger any process to run."""
        subprocess.call(['/bin/true'])


if __name__ == "__main__":
    HelloWorld()
