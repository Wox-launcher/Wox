const React = require('react');
const Reconciler = require('react-reconciler');
const { DefaultEventPriority } = require('react-reconciler/constants');

// Keep React state inside Node; only the mapped Wox query response crosses the process boundary.
function createRenderer(onCommit, onError) {
  const container = { children: [] };
  const context = {};
  let priority = DefaultEventPriority;
  let nextId = 0;
  const remove = (parent, child) => {
    const index = parent.children.indexOf(child);
    if (index >= 0) parent.children.splice(index, 1);
  };
  const append = (parent, child) => { remove(parent, child); parent.children.push(child); };
  const insert = (parent, child, before) => { remove(parent, child); parent.children.splice(parent.children.indexOf(before), 0, child); };
  const reconciler = Reconciler({
    supportsMutation: true, supportsPersistence: false, supportsHydration: false,
    isPrimaryRenderer: true,
    getRootHostContext: () => context, getChildHostContext: () => context,
    getPublicInstance: instance => instance,
    createInstance: (type, props) => ({ type, props, children: [], id: String(++nextId) }),
    createTextInstance: text => ({ type: '#text', text, children: [] }),
    appendInitialChild: append, appendChild: append, appendChildToContainer: append,
    insertBefore: insert, insertInContainerBefore: insert,
    removeChild: remove, removeChildFromContainer: remove,
    clearContainer: node => { node.children = []; },
    finalizeInitialChildren: () => false, shouldSetTextContent: () => false,
    prepareForCommit: () => null, resetAfterCommit: () => onCommit(container),
    commitUpdate: (instance, type, oldProps, props) => { instance.props = props; },
    commitTextUpdate: (instance, oldText, text) => { instance.text = text; },
    resetTextContent: () => {}, commitMount: () => {}, detachDeletedInstance: () => {},
    preparePortalMount: () => {},
    scheduleTimeout: setTimeout, cancelTimeout: clearTimeout, noTimeout: -1,
    supportsMicrotasks: true, scheduleMicrotask: queueMicrotask,
    getCurrentUpdatePriority: () => priority,
    setCurrentUpdatePriority: value => { priority = value; },
    resolveUpdatePriority: () => priority || DefaultEventPriority,
    shouldAttemptEagerTransition: () => false,
    maySuspendCommit: () => false, preloadInstance: () => true,
    startSuspendingCommit: () => {}, suspendInstance: () => {}, waitForCommitToBeReady: () => null,
    NotPendingTransition: null, HostTransitionContext: React.createContext(null),
    resetFormInstance: () => {}, requestPostPaintCallback: () => {},
    resolveEventType: () => null, resolveEventTimeStamp: () => -1.1,
    trackSchedulerEvent: () => {}
  });
  const root = reconciler.createContainer(container, 1, null, false, null, '', onError, onError, onError, null);
  return {
    render: element => reconciler.updateContainer(element, root, null, null),
    unmount: () => reconciler.updateContainer(null, root, null, null)
  };
}

// Element-valued props must participate in reconciliation so hooks inside actions/details run normally.
function component(type) {
  return props => {
    const { children, ...rest } = props;
    const slots = [];
    for (const key of ['actions', 'detail', 'metadata', 'searchBarAccessory']) {
      if (rest[key] != null) {
        slots.push(React.createElement('slot', { key, name: key }, rest[key]));
        delete rest[key];
      }
    }
    return React.createElement(type, rest, children, ...slots);
  };
}

module.exports = { createRenderer, component };
