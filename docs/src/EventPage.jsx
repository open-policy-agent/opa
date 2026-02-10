import Heading from "@theme/Heading";
import Layout from "@theme/Layout";
import React, { useEffect, useState } from "react";

import AgendaItem from "./components/Event/AgendaItem";
import Countdown from "./components/Event/Countdown";

import eventsData from "@generated/events-data/default/events.json";

import styles from "./EventPage.module.css";

const EventPage = (props) => {
  const eventId = props.route.customData.id;
  const event = eventsData[eventId];

  // Pages are only created for events that exist in data
  if (!event || !event.agenda) {
    return null;
  }

  const [eventStatus, setEventStatus] = useState("before");

  const eventStart = new Date(event.startDate);
  const MS_PER_DAY = 1000 * 60 * 60 * 24;
  const eventEnd = new Date(new Date(event.endDate).getTime() + MS_PER_DAY);

  useEffect(() => {
    const updateStatus = () => {
      const now = new Date();

      if (now >= eventStart && now < eventEnd) {
        setEventStatus("during");
      } else if (now >= eventEnd) {
        setEventStatus("after");
      } else {
        setEventStatus("before");
      }
    };

    updateStatus();
    const interval = setInterval(updateStatus, 1000);
    return () => clearInterval(interval);
  }, [eventStart, eventEnd]);

  const bannerUrl = `/img/event-banners/${event.banner}`;

  return (
    <Layout title={event.title}>
      <div className={styles.pageLayout}>
        <div className={styles.sidebar}>
          <img
            src={bannerUrl}
            alt={event.title}
            className={styles.banner}
          />

          {event.location && (
            <div className={styles.eventLocation}>
              {event.location}
            </div>
          )}

          {eventStatus === "before" && <Countdown targetDate={eventStart} title={event.title} />}

          {eventStatus === "during" && (
            <Heading as="h1" className={styles.eventMessage}>
              {event.title} is on now
            </Heading>
          )}

          {eventStatus === "after" && (
            <Heading as="h1" className={styles.eventMessage}>
              {event.title} has now passed
            </Heading>
          )}
        </div>

        <div className={styles.mainContent}>
          <div className={styles.section}>
            <Heading as="h2" className={styles.sectionTitle}>
              Agenda
            </Heading>
            {event.agenda.map((dayAgenda, dayIndex) => (
              <div key={dayIndex} className={styles.daySection}>
                <h3 className={styles.dayTitle}>
                  {dayAgenda.day}, {dayAgenda.date}
                </h3>
                <div className={styles.dayItems}>
                  {dayAgenda.items.map((item, itemIndex) => (
                    <AgendaItem
                      key={itemIndex}
                      item={item}
                    />
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </Layout>
  );
};

export default EventPage;
