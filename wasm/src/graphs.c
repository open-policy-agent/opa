#include "graphs.h"
#include "std.h"

static void __builtin_graph_reachable(opa_value *edges, opa_array_t *queue, opa_set_t *reached)
{
    switch (opa_value_type(edges))
    {
        case OPA_SET:
        {
            opa_set_t *x = opa_cast_set(edges);

            for (int i = 0; i < x->n; i++)
            {
                opa_set_elem_t *elem = x->buckets[i];

                while (elem != NULL)
                {
                    if (reached == NULL || opa_set_get(reached, elem->v) == NULL)
                    {
                        opa_array_append(queue, elem->v);
                    }
                    elem = elem->next;
                }
            }
            break;
        }  
        case OPA_ARRAY: 
        {
            opa_array_t *y = opa_cast_array(edges);

            for (int i = 0; i < y->len; i++)
            {
                opa_value *elem = y->elems[i].v;

                if (reached == NULL || opa_set_get(reached, elem) == NULL)
                {
                    opa_array_append(queue, elem);
                }
            }
            break;
        }
    }
}

OPA_BUILTIN
opa_value *builtin_graph_reachable(opa_value *graph, opa_value *initial)
{
    if (opa_value_type(graph) != OPA_OBJECT)
    {
        return NULL;
    }
    if (opa_value_type(initial) != OPA_SET && opa_value_type(initial) != OPA_ARRAY)
    {
        return NULL;
    }

    // This is a queue that holds all nodes we still need to visit. It is
    // initialized to the initial set of nodes we start out with.
    opa_array_t *queue = opa_cast_array(opa_array());

    if (initial != NULL)
    {
        __builtin_graph_reachable(initial, queue, NULL);
    }

    // This is the set of nodes we have reached.
    opa_set_t *reached = opa_cast_set(opa_set());

    // Keep going as long as we have nodes in the queue.
    for (int index = 0; index < queue->len; index++)
    {
        // Get the edges for this node.
        opa_value *node = queue->elems[index].v;
        opa_value *edges = opa_value_get(graph, node);

        if (edges != NULL)
        {
            __builtin_graph_reachable(edges, queue, reached);

            // Mark current node as reached.
            opa_set_add(reached, node);
        }
    }

    return &reached->hdr;
}
