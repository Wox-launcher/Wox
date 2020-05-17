# -*- coding: utf-8 -*-

from wox import Wox

class HelloWorld(Wox):

    # query is default function to receive realtime keystrokes from wox launcher
    def query(self, query):
        results = []
        results.append({
            "Title": "Hello World",
            "SubTitle": "Query: {}".format(query),
            "IcoPath":"Images/app.png",
            "ContextData": "ctxData",
            "JsonRPCAction": {
                'method': 'take_action',
                'parameters': ["{}".format("SomeData")],
                'dontHideAfterAction': False
            }
        })
        return results

    # context_menu is default function called for ContextData where `data = ctxData`
    def context_menu(self, data):
        results = []
        results.append({
            "Title": "Context menu entry",
            "SubTitle": "Data: {}".format(data),
            "IcoPath":"Images/app.png"
        })
        return results

    def take_action(self, SomeArgument):
        # Choose what to trigger on pressing enter on the result.
        # use SomeArgument to do something with data sent by parameters.

        return None

if __name__ == "__main__":
    HelloWorld()
