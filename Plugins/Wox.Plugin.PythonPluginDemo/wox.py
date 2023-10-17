from abc import ABCMeta, abstractmethod


class PublicAPI:
    def __init__(self, proxy):
        self.proxy = proxy

    def change_query(self, query):
        self.proxy.change_query(query)

    def hide_app(self):
        self.proxy.hide_app()

    def show_app(self):
        self.proxy.show_app()

    def show_msg(self, title, description="", icon_path=""):
        self.proxy.show_msg(title, description, icon_path)

    def log(self, msg):
        self.proxy.log(msg)

    def get_translation(self, key):
        return self.proxy.get_translation(key)


class PluginInitContext:
    api: PublicAPI

    def __init__(self, api: PublicAPI):
        self.api = api


class Query:
    def __init__(self, raw_query, trigger_keyword, command, search):
        self.raw_query = raw_query
        self.trigger_keyword = trigger_keyword
        self.command = command
        self.search = search


class WoxPlugin(metaclass=ABCMeta):
    @abstractmethod
    def init(self, init_context: PluginInitContext):
        pass

    @abstractmethod
    def query(self, query: Query):
        pass
