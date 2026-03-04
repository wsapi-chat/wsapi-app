package event

import (
	"context"

	"github.com/wsapi-chat/wsapi-app/internal/identity"
	"go.mau.fi/whatsmeow/store"
	waTypes "go.mau.fi/whatsmeow/types"
)

// lidsFromCtx safely extracts the LID store, handling nil pctx.
func lidsFromCtx(pctx *ProjectorContext) store.LIDStore {
	if pctx == nil {
		return nil
	}
	return pctx.LIDs()
}

// resolveIdentity resolves a JID to an Identity using the projector context's LID store.
func resolveIdentity(jid waTypes.JID, pctx *ProjectorContext) identity.Identity {
	return identity.Resolve(context.Background(), jid, waTypes.EmptyJID, lidsFromCtx(pctx))
}

// resolveIdentityWithAlt resolves a JID to an Identity using an alternate JID.
func resolveIdentityWithAlt(jid, altJID waTypes.JID, pctx *ProjectorContext) identity.Identity {
	return identity.Resolve(context.Background(), jid, altJID, lidsFromCtx(pctx))
}

// resolveSender resolves a JID to a Sender using the projector context's LID store.
func resolveSender(jid waTypes.JID, isMe bool, altJID waTypes.JID, pctx *ProjectorContext) identity.Sender {
	return identity.ResolveSender(context.Background(), jid, isMe, altJID, lidsFromCtx(pctx))
}

// resolveMentions resolves a slice of JID strings, converting any LID-based JIDs
// to phone-based JID strings when possible. Returns the original slice unchanged
// if no LID store is available.
func resolveMentions(jidStrs []string, pctx *ProjectorContext) []string {
	lids := lidsFromCtx(pctx)
	if lids == nil {
		return jidStrs
	}

	result := make([]string, 0, len(jidStrs))
	for _, s := range jidStrs {
		jid, err := waTypes.ParseJID(s)
		if err != nil || jid.Server != waTypes.HiddenUserServer {
			result = append(result, s)
			continue
		}
		if pn, err := lids.GetPNForLID(context.Background(), jid); err == nil && pn.User != "" {
			result = append(result, waTypes.JID{User: pn.User, Server: waTypes.DefaultUserServer}.String())
		} else {
			result = append(result, s)
		}
	}
	return result
}

// resolveMembers resolves a slice of JIDs to Identity objects.
func resolveMembers(jids []waTypes.JID, pctx *ProjectorContext) []identity.Identity {
	result := make([]identity.Identity, 0, len(jids))
	lids := lidsFromCtx(pctx)
	for _, jid := range jids {
		result = append(result, identity.Resolve(context.Background(), jid, waTypes.EmptyJID, lids))
	}
	return result
}
