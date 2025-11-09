# Migration & Fix Guide

## What was fixed

### 1. **Broken wirefilter expressions** (Critical bug causing hangs)
   - **Problem**: The original Go implementation used `{"lists": "auto"}` which is not valid Cloudflare Gateway wirefilter syntax
   - **Fix**: Now generates proper wirefilter expressions: `any(dns.domains[*] in $listID) or any(dns.domains[*] in $listID2) or ...`
   - **Impact**: Rules now actually work and reference the created lists correctly

### 2. **No cleanup of old resources** (Caused orphaned lists and duplicate rules)
   - **Problem**: When renaming from "CGPS" to "Go-CFGW", old resources were left behind, causing conflicts
   - **Fix**: Automatically deletes ALL old resources (both CGPS and Go-CFGW) before creating new ones
   - **Impact**: 
     - Safe to restart after rate limits or connection failures
     - No orphaned resources in Cloudflare dashboard
     - Idempotent operation (can run multiple times safely)

### 3. **Missing SNI support**
   - **Problem**: The Node.js version supports SNI-based filtering but Go version didn't
   - **Fix**: Added full SNI support with `BLOCK_BASED_ON_SNI` environment variable
   - **Impact**: Feature parity with Node.js implementation

### 4. **Incomplete rule descriptions**
   - **Problem**: Rules had generic "Managed by go-cfgw" description
   - **Fix**: Now uses proper description matching Node.js: "Filter lists created by go-cfgw. Avoid editing this rule. Changing the name of this rule will break the script."
   - **Impact**: Better user guidance and consistency with Node.js version

## How cleanup works

Every time go-cfgw runs, it:

1. **Deletes old rules** that match:
   - "CGPS Filter Lists" (Node.js version)
   - "CGPS Filter Lists - SNI Based Filtering" (Node.js SNI version)
   - "Go-CFGW Filter Lists" (any previous Go runs)
   - "Go-CFGW Filter Lists - SNI Based Filtering" (any previous Go SNI runs)

2. **Deletes old lists** that match:
   - "CGPS List - Chunk N" (Node.js version)
   - "Go-CFGW Block List - Chunk N" (any previous Go runs)
   - "Go-CFGW Allow List - Chunk N" (any previous Go runs)

3. **Waits 2 seconds** for Cloudflare API to settle

4. **Creates new lists** with fresh data

5. **Creates new rules** with proper wirefilter expressions referencing the new list IDs

## Why this prevents hangs

The original issue where the program "hangs indefinitely" was caused by:

1. Invalid wirefilter expression (`{"lists": "auto"}`) causing Cloudflare API to reject or mishandle the rule
2. Orphaned lists with no valid rule referencing them
3. Duplicate resources causing conflicts

With the fixes:
- Proper wirefilter expressions ensure rules are created successfully
- Cleanup ensures no duplicate or orphaned resources
- Idempotent operation means safe restarts

## Migrating from Node.js cloudflare-gateway-pihole-scripts

### Before first run

No special steps needed! The Go version will automatically:
- Detect and delete old CGPS resources
- Create new Go-CFGW resources
- Set up proper rules

### After first run

Your Cloudflare dashboard will show:
- **Lists**: "Go-CFGW Block List - Chunk 1", "Go-CFGW Block List - Chunk 2", etc.
- **Rules**: "Go-CFGW Filter Lists" (and optionally "Go-CFGW Filter Lists - SNI Based Filtering")
- **Old resources**: All "CGPS" resources automatically deleted

### Environment variables

Most environment variables are the same:
- ✅ `CLOUDFLARE_API_TOKEN` - same
- ✅ `CLOUDFLARE_ACCOUNT_ID` - same
- ✅ `CLOUDFLARE_LIST_ITEM_LIMIT` - same
- ✅ `BLOCK_PAGE_ENABLED` - same
- ✅ `BLOCK_BASED_ON_SNI` - same
- ✅ `ALLOWLIST_URLS` / `USER_DEFINED_ALLOWLIST_URLS` - both supported
- ✅ `BLOCKLIST_URLS` / `USER_DEFINED_BLOCKLIST_URLS` - both supported

## Troubleshooting

### "Program still hangs"

If you experience hanging after this fix:

1. **Check Cloudflare API token permissions**: Ensure it has Gateway List and Rule scopes
2. **Check rate limits**: Look for "rate limited" messages in logs
3. **Check network connectivity**: Ensure you can reach api.cloudflare.com
4. **Enable debug mode**: Set `DRY_RUN=1` to test without making changes
5. **Manual cleanup**: If needed, manually delete all old lists/rules in Cloudflare dashboard, then run go-cfgw

### "Old CGPS resources still present"

The cleanup should handle this automatically. If you still see old resources:

1. Run go-cfgw again - it will clean them up
2. If persistent, manually delete via Cloudflare dashboard
3. Check if resources are referenced by other rules (not created by CGPS/go-cfgw)

### "Rate limited errors"

The client implements exponential backoff and respects `Retry-After` headers. If you hit rate limits:

1. Wait a few minutes and run again
2. The cleanup phase happens first, so rate limits during creation won't leave orphans
3. Consider reducing `CLOUDFLARE_LIST_ITEM_LIMIT` if you have a large blocklist

## Technical details

### Wirefilter expression format

Node.js generates:
```javascript
"any(dns.domains[*] in $listID1) or any(dns.domains[*] in $listID2) or ..."
```

Go now generates the exact same format:
```go
"any(dns.domains[*] in $listID1) or any(dns.domains[*] in $listID2) or ..."
```

This is the correct Cloudflare Gateway wirefilter syntax for matching domains against multiple lists.

### Cleanup logic

The cleanup functions in `internal/cf/client.go`:

- `DeleteAllOldRules()`: Queries all rules, filters by name pattern, deletes matches
- `DeleteAllOldLists()`: Queries all lists, filters by name pattern, deletes matches

Both use:
- `strings.Contains()` for rule matching (matches partial names like "CGPS Filter Lists" in "CGPS Filter Lists - SNI")
- `strings.HasPrefix()` for list matching (exact prefix match)
- Continue on error (logs warning but doesn't stop if one deletion fails)
- Returns total count of deleted resources

### Idempotency

Every run:
1. Cleans up old resources (idempotent - safe if none exist)
2. Creates new resources (fresh state every time)
3. If interrupted, next run will clean up partial state and start fresh

This design ensures:
- No accumulation of orphaned resources
- Safe restarts after any failure
- Consistent state regardless of previous runs
- No manual intervention needed

## Verification

After running go-cfgw, verify in your Cloudflare dashboard:

1. **Zero Trust > Gateway > Firewall Policies**: 
   - Should see "Go-CFGW Filter Lists" rule
   - If SNI enabled, also see "Go-CFGW Filter Lists - SNI Based Filtering"
   - Should NOT see any "CGPS Filter Lists" rules

2. **Zero Trust > Gateway > Lists**:
   - Should see "Go-CFGW Block List - Chunk N" entries
   - Should NOT see any "CGPS List - Chunk N" entries

3. **Rule details** (click on "Go-CFGW Filter Lists"):
   - Traffic condition should show: `any(dns.domains[*] in $[list-id]) or any(dns.domains[*] in $[list-id]) or ...`
   - Should reference all created list IDs
   - Action should be "Block"
   - Enabled should be "Yes"

If you see the above, the migration was successful!
