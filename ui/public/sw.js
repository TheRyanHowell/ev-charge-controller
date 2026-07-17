self.addEventListener('install', (event) => {
  self.skipWaiting()
})

self.addEventListener('activate', (event) => {
  event.waitUntil(clients.claim())
})

// The push handler only displays the notification. Subscription lifecycle is
// handled by the pushsubscriptionchange handler below and by the app-open
// sync in the page (ensurePushSubscription) - tearing down and recreating the
// subscription from inside the push handler proved destructive: any failure
// mid-rotation left the device silently unsubscribed.
self.addEventListener('push', (event) => {
  const data = event.data ? event.data.json() : {}
  const title = data.title || 'EV Charge'
  const body = data.body || ''
  const vibration = data.vibration || [300, 100, 300, 100, 300, 100, 300, 100, 600, 200]

  event.waitUntil(
    self.registration.showNotification(title, {
      body,
      icon: '/favicon.ico',
      badge: '/favicon.ico',
      vibrate: vibration,
      requireInteraction: true,
      tag: title,
    })
  )
})

// pushsubscriptionchange fires when the push service rotates, refreshes, or
// invalidates the subscription. Resubscribe with the same application server
// key and re-register with the backend so delivery continues without the app
// having to be opened.
self.addEventListener('pushsubscriptionchange', (event) => {
  event.waitUntil(renewSubscription(event))
})

function renewSubscription(event) {
  const oldSubscription = event.oldSubscription || null
  const oldEndpoint = oldSubscription ? oldSubscription.endpoint : null

  // Some browsers hand over the replacement subscription directly.
  const subscriptionPromise = event.newSubscription
    ? Promise.resolve(event.newSubscription)
    : getApplicationServerKey(oldSubscription).then(function (applicationServerKey) {
        return self.registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: applicationServerKey,
        })
      })

  return subscriptionPromise
    .then(function (newSubscription) {
      return registerSubscription(newSubscription).then(function () {
        if (oldEndpoint && oldEndpoint !== newSubscription.endpoint) {
          return unregisterEndpoint(oldEndpoint)
        }
      })
    })
    .catch(function (error) {
      // Best effort: the app-open sync (ensurePushSubscription) repairs the
      // registration the next time the PWA is opened.
      console.error('[SW] Failed to renew push subscription:', error)
    })
}

function getApplicationServerKey(oldSubscription) {
  if (oldSubscription && oldSubscription.options && oldSubscription.options.applicationServerKey) {
    return Promise.resolve(oldSubscription.options.applicationServerKey)
  }
  return fetch('/api/push-subscriptions')
    .then(function (res) {
      return res.json()
    })
    .then(function (data) {
      return data.publicKey ? urlBase64ToBufferKey(data.publicKey) : undefined
    })
}

function registerSubscription(subscription) {
  const keys = subscription.toJSON().keys || {}
  return fetch('/api/push-subscriptions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      endpoint: subscription.endpoint,
      p256dhKey: keys.p256dh || '',
      authKey: keys.auth || '',
    }),
  })
}

function unregisterEndpoint(endpoint) {
  return fetch('/api/push-subscriptions', {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ endpoint: endpoint }),
  })
}

function urlBase64ToBufferKey(base64String) {
  var padding = '='.repeat((4 - (base64String.length % 4)) % 4)
  var base64 = base64String.replace(/-/g, '+').replace(/_/g, '/') + padding
  var binaryString = atob(base64)
  var bytes = new Uint8Array(binaryString.length)
  for (var i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i)
  }
  return bytes.buffer
}

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil(
    clients.matchAll({ type: 'window' }).then((matches) => {
      if (matches.length > 0) {
        return matches[0].focus()
      }
      return clients.openWindow('/')
    }),
  )
})
